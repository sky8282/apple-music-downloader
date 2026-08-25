package runv4

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
	"sync"
	"golang.org/x/sync/errgroup"

	"main/utils/structs"
	"main/utils/runv14"

	"github.com/grafov/m3u8"
	"github.com/itouakirai/mp4ff/mp4"
)

var (
	globalDecryptMu sync.Mutex
	activeDecrypts  int
	decryptCond     = sync.NewCond(&globalDecryptMu)
)

const prefetchKey = "skd://itunes.apple.com/P000000000/s1/e1"
var ErrTimeout = errors.New("response timed out")

type TimedResponseBody struct {
	timeout   time.Duration
	timer     *time.Timer
	threshold int
	body      io.Reader
}

type decryptJob struct {
	Seq       int
	Frag      *mp4.Fragment
	Tmpl      *template
	RawOffset int64
}

type decryptResult struct {
	Seq       int
	Frag      *mp4.Fragment
	RawOffset int64
}

type progressWriter struct {
	total      int64
	current    int64
	ch         chan runv14.ProgressUpdate
	stage      string
	lastReport time.Time
	lastBytes  int64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.current += int64(n)
	now := time.Now()
	if now.Sub(pw.lastReport) >= 100*time.Millisecond || pw.current >= pw.total {
		speed := 0.0
		if elapsed := now.Sub(pw.lastReport).Seconds(); elapsed > 0 {
			speed = float64(pw.current-pw.lastBytes) / elapsed
		}
		pct := int(float64(pw.current) * 100 / float64(pw.total))
		if pct > 100 {
			pct = 100
		}
		if pw.ch != nil {
			if pct >= 100 {
				select {
				case pw.ch <- runv14.ProgressUpdate{Percentage: pct, SpeedBPS: speed, Stage: pw.stage}:
				case <-time.After(1 * time.Second):
				}
			} else {
				select {
				case pw.ch <- runv14.ProgressUpdate{Percentage: pct, SpeedBPS: speed, Stage: pw.stage}:
				default:
				}
			}
		}
		pw.lastReport = now
		pw.lastBytes = pw.current
	}
	return n, nil
}

func (b *TimedResponseBody) Read(p []byte) (int, error) {
	n, err := b.body.Read(p)
	if err != nil {
		return n, err
	}
	if n >= b.threshold {
		b.timer.Reset(b.timeout)
	}
	return n, err
}

const (
	downloadMaxAttempts = 5                 
	downloadIdleTimeout = 30 * time.Second  
)

func downloadWithResume(ctx context.Context, client *http.Client, fileUrl string,
	header http.Header, totalLen int64, progressChan chan runv14.ProgressUpdate, tempFilePath string) error {

	file, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	var offset int64 = stat.Size()

	if offset > totalLen {
		file.Truncate(0)
		offset = 0
	} else if offset > 0 {
		file.Seek(offset, io.SeekStart)
	}

	if offset == totalLen {
		return nil
	}

	backoff := 2 * time.Second

	for attempt := 1; attempt <= downloadMaxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}

		attemptCtx, attemptCancel := context.WithCancel(ctx)
		req, err := http.NewRequestWithContext(attemptCtx, "GET", fileUrl, nil)
		if err != nil {
			attemptCancel()
			return err
		}
		req.Header = header
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}

		resp, err := client.Do(req)
		if err != nil {
			attemptCancel()
			continue
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			attemptCancel()
			return fmt.Errorf("download failed: server returned %s", resp.Status)
		}
		if offset > 0 && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			attemptCancel()
			return errors.New("server does not support Range requests, cannot resume")
		}

		timer := time.AfterFunc(downloadIdleTimeout, attemptCancel)
		body := &TimedResponseBody{
			timeout:   downloadIdleTimeout,
			timer:     timer,
			threshold: 1,
			body:      resp.Body,
		}

		pw := &progressWriter{
			total:      totalLen,
			current:    offset,
			ch:         progressChan,
			stage:      "download",
			lastReport: time.Now(),
			lastBytes:  offset,
		}

		n, copyErr := io.Copy(io.MultiWriter(file, pw), body)
		resp.Body.Close()
		timer.Stop()
		attemptCancel()
		offset += n

		if copyErr == nil && offset == totalLen {
			if progressChan != nil {
				select {
				case progressChan <- runv14.ProgressUpdate{Percentage: 100, SpeedBPS: 0, Stage: "download"}:
				case <-time.After(1 * time.Second):
				}
			}
			return nil
		}
		
		if copyErr == nil {
			copyErr = fmt.Errorf("short download: got %d of %d bytes", offset, totalLen)
		}
	}
	return fmt.Errorf("download failed after %d attempts (got %d/%d bytes)",
		downloadMaxAttempts, offset, totalLen)
}

func Run(adamId string, playlistUrl string, outfile string, account *structs.Account, Config structs.ConfigSet, progressChan chan runv14.ProgressUpdate) error {
	var err error
	var optstimeout uint
	optstimeout = 0
	timeout := time.Duration(optstimeout * uint(time.Millisecond))
	header := make(http.Header)

	req, err := http.NewRequest("GET", playlistUrl, nil)
	if err != nil {
		return err
	}
	req.Header = header
	do, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return err
	}

	segments, err := parseMediaPlaylist(do.Body)
	if err != nil {
		return err
	}
	segment := segments[0]
	if segment == nil {
		return errors.New("no segments extracted from playlist")
	}
	if segment.Limit <= 0 {
		return errors.New("non-byterange playlists are currently unsupported")
	}

	parsedUrl, err := url.Parse(playlistUrl)
	if err != nil {
		return err
	}
	fileUrl, err := parsedUrl.Parse(segment.URI)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	req, err = http.NewRequestWithContext(ctx, "GET", fileUrl.String(), nil)
	if err != nil {
		return err
	}
	req.Header = header

	http.DefaultClient.Timeout = 15 * time.Second

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
	}
	
	do, err = client.Do(req)
	if err != nil {
		return err
	}
	var totalLen int64 = do.ContentLength
	do.Body.Close()

	tempEncFile := outfile + ".enc"
	err = downloadWithResume(ctx, client, fileUrl.String(), header, totalLen, progressChan, tempEncFile)
	if err != nil {
		return err
	}
	defer os.Remove(tempEncFile)

	encFile, err := os.Open(tempEncFile)
	if err != nil {
		return err
	}
	defer encFile.Close()

	keyServer := account.KeyServer

	err = downloadAndDecryptFile(keyServer, encFile, outfile, adamId, segments, totalLen, Config, progressChan)
	if err != nil {
		return err
	}
	return nil
}

func downloadAndDecryptFile(keyServer string, in io.Reader, outfile string,
	adamId string, playlistSegments []*m3u8.MediaSegment, totalLen int64, Config structs.ConfigSet, progressChan chan runv14.ProgressUpdate) error {
	
	globalDecryptMu.Lock()
	limit := 1
	if !Config.EnableQueueDecrypt {
		limit = Config.DecryptThreads
		if limit <= 0 {
			limit = 10
		}
	}
	
	for activeDecrypts >= limit {
		decryptCond.Wait()
	}
	activeDecrypts++
	globalDecryptMu.Unlock()

	defer func() {
		globalDecryptMu.Lock()
		activeDecrypts--
		decryptCond.Broadcast()
		globalDecryptMu.Unlock()
	}()

	ofh, err := os.Create(outfile)
	if err != nil {
		return err
	}
	defer ofh.Close()
	
	bufferSize := Config.BufferSizeKB * 1024
	if bufferSize <= 0 {
		bufferSize = 4096 * 1024 
	}
	outBuf := bufio.NewWriterSize(ofh, bufferSize)
	inBuf := bufio.NewReaderSize(in, bufferSize)
	
	if progressChan != nil {
		select {
		case progressChan <- runv14.ProgressUpdate{Percentage: 0, SpeedBPS: 0, Stage: "decrypt"}:
		case <-time.After(1 * time.Second):
		}
	}

	init, offset, err := ReadInitSegment(inBuf)
	if err != nil {
		return err
	}
	if init == nil {
		return errors.New("no init segment found")
	}

	tracks, err := TransformInit(init)
	if err != nil {
		return err
	}
	err = sanitizeInit(init)
	if err != nil {
	}
	err = init.Encode(outBuf)
	if err != nil {
		return err
	}

	var currentOffset int64 = int64(offset)
	var lastBytes int64 = currentOffset
	lastReport := time.Now()

	reportProgress := func(added int64) {
		currentOffset += added
		now := time.Now()
		if now.Sub(lastReport) >= 100*time.Millisecond || currentOffset >= totalLen {
			speed := 0.0
			if elapsed := now.Sub(lastReport).Seconds(); elapsed > 0 {
				speed = float64(currentOffset-lastBytes) / elapsed
			}
			pct := int(float64(currentOffset) * 100 / float64(totalLen))
			if pct > 100 {
				pct = 100
			}
			if progressChan != nil {
				if pct >= 100 {
					select {
					case progressChan <- runv14.ProgressUpdate{Percentage: pct, SpeedBPS: speed, Stage: "decrypt"}:
					case <-time.After(1 * time.Second):
					}
				} else {
					select {
					case progressChan <- runv14.ProgressUpdate{Percentage: pct, SpeedBPS: speed, Stage: "decrypt"}:
					default:
					}
				}
			}
			lastReport = now
			lastBytes = currentOffset
		}
	}
	
	reportProgress(0)

	eg, ctx := errgroup.WithContext(context.Background())

	workerCount := 10
	if Config.DecryptThreads > 0 {
		workerCount = Config.DecryptThreads
	}

	jobBufferSize := workerCount * 2
	if jobBufferSize < 20 {
		jobBufferSize = 20
	}
	
	var inFlightSem chan struct{}
	if Config.EnableQueueDecrypt {
		inFlightSem = make(chan struct{}, jobBufferSize)
	} else {
		jobBufferSize = 10000 
	}

	jobs := make(chan *decryptJob, jobBufferSize)
	results := make(chan *decryptResult, jobBufferSize)

	eg.Go(func() error {
		buffer := make(map[int]*decryptResult)
		expectedSeq := 0

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case res, ok := <-results:
				if !ok {
					return nil
				}
				
				buffer[res.Seq] = res

				for {
					if readyRes, exists := buffer[expectedSeq]; exists {
						if err := readyRes.Frag.Encode(outBuf); err != nil {
							return fmt.Errorf("encode fragment seq %d failed: %w", expectedSeq, err)
						}
						reportProgress(readyRes.RawOffset)
						
						delete(buffer, expectedSeq)
						expectedSeq++
						
						if inFlightSem != nil {
							<-inFlightSem
						}
					} else {
						break
					}
				}
			}
		}
	})

	var workerWg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		workerWg.Add(1)
		eg.Go(func() error {
			defer workerWg.Done()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case job, ok := <-jobs:
					if !ok {
						return nil
					}
					
					err := DecryptFragment(job.Frag, tracks, job.Tmpl)
					if err != nil {
						return fmt.Errorf("decryptFragment seq %d: %w", job.Seq, err)
					}
					
					select {
					case results <- &decryptResult{
						Seq:       job.Seq,
						Frag:      job.Frag,
						RawOffset: job.RawOffset,
					}:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		})
	}

	eg.Go(func() error {
		workerWg.Wait()
		close(results)
		return nil
	})

	eg.Go(func() error {
		defer close(jobs)
		seq := 0
		var tmpl *template

		for i := 0; ; i++ {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			if inFlightSem != nil {
				select {
				case inFlightSem <- struct{}{}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			var frag *mp4.Fragment
			rawoffset := offset
			frag, offset, err = ReadNextFragment(inBuf, offset)
			rawoffset = offset - rawoffset
			if err != nil {
				if inFlightSem != nil {
					<-inFlightSem
				}
				return fmt.Errorf("read fragment: %w", err)
			}
			if frag == nil {
				if inFlightSem != nil {
					<-inFlightSem 
				}
				break 
			}

			if i < len(playlistSegments) {
				segment := playlistSegments[i]
				if segment != nil && segment.Key != nil && i < 2 {
					if segment.Key.URI == prefetchKey {
						tmpl = prefetchTemplate()
					} else {
						tmpl, err = fetchTemplate(keyServer, adamId, segment.Key.URI)
					}
					if err != nil {
						if inFlightSem != nil {
							<-inFlightSem
						}
						return err
					}
				}
			}

			job := &decryptJob{
				Seq:       seq,
				Frag:      frag,
				Tmpl:      tmpl,
				RawOffset: int64(rawoffset),
			}

			select {
			case jobs <- job:
			case <-ctx.Done():
				return ctx.Err()
			}
			seq++
		}
		return nil
	})

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	err = outBuf.Flush()
	if err != nil {
		return err
	}
	
	if progressChan != nil {
		select {
		case progressChan <- runv14.ProgressUpdate{Percentage: 100, SpeedBPS: 0, Stage: "decrypt"}:
		case <-time.After(1 * time.Second):
		}
	}
	
	return nil
}







func sanitizeInit(init *mp4.InitSegment) error {
	traks := init.Moov.Traks
	if len(traks) > 1 {
		return errors.New("more than 1 track found")
	}
	stsd := traks[0].Mdia.Minf.Stbl.Stsd
	if stsd.SampleCount == 1 {
		return nil
	}
	if stsd.SampleCount > 2 {
		return fmt.Errorf("expected only 1 or 2 entries in stsd, got %d", stsd.SampleCount)
	}
	children := stsd.Children
	if children[0].Type() != children[1].Type() {
		return errors.New("children in stsd are not of the same type")
	}
	stsd.Children = children[:1]
	stsd.SampleCount = 1
	return nil
}

func filterResponse(f io.Reader) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	scanner := bufio.NewScanner(f)

	prefix := []byte("#EXT-X-KEY:")
	keyFormat := []byte("streamingkeydelivery")
	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if bytes.HasPrefix(lineBytes, prefix) && !bytes.Contains(lineBytes, keyFormat) {
			continue
		}
		_, err := buf.Write(lineBytes)
		if err != nil {
			return nil, err
		}
		_, err = buf.WriteString("\n")
		if err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return buf, nil
}

func parseMediaPlaylist(r io.ReadCloser) ([]*m3u8.MediaSegment, error) {
	defer r.Close()
	playlistBuf, err := filterResponse(r)
	if err != nil {
		return nil, err
	}

	playlist, listType, err := m3u8.Decode(*playlistBuf, true)
	if err != nil {
		return nil, err
	}

	if listType != m3u8.MEDIA {
		return nil, errors.New("m3u8 not of media type")
	}

	mediaPlaylist := playlist.(*m3u8.MediaPlaylist)
	return mediaPlaylist.Segments, nil
}

func ReadInitSegment(r io.Reader) (*mp4.InitSegment, uint64, error) {
	var offset uint64 = 0
	init := mp4.NewMP4Init()
	for i := 0; i < 2; i++ {
		box, err := mp4.DecodeBox(offset, r)
		if err != nil {
			return nil, offset, err
		}
		boxType := box.Type()
		if boxType != "ftyp" && boxType != "moov" {
			return nil, offset, fmt.Errorf("unexpected box type %s, should be ftyp or moov", boxType)
		}
		init.AddChild(box)
		offset += box.Size()
	}
	return init, offset, nil
}

func ReadNextFragment(r io.Reader, offset uint64) (*mp4.Fragment, uint64, error) {
	frag := mp4.NewFragment()
	for {
		box, err := mp4.DecodeBox(offset, r)
		if err == io.EOF {
			return nil, offset, nil
		}
		if err != nil {
			return nil, offset, err
		}
		boxType := box.Type()
		offset += box.Size()
		if boxType == "moof" || boxType == "emsg" || boxType == "prft" {
			frag.AddChild(box)
			continue
		}
		if boxType == "mdat" {
			frag.AddChild(box)
			break
		}
		fmt.Printf("ignoring a %s box found mid-stream", boxType)
	}
	if frag.Moof == nil {
		return nil, offset, fmt.Errorf("more than one mdat box in fragment (box ends @ offset %d)", offset)
	}
	return frag, offset, nil
}

func FilterSbgpSgpd(children []mp4.Box) ([]mp4.Box, uint64) {
	var bytesRemoved uint64 = 0
	remainingChildren := make([]mp4.Box, 0, len(children))
	for _, child := range children {
		switch box := child.(type) {
		case *mp4.SbgpBox:
			if box.GroupingType == "seam" || box.GroupingType == "seig" {
				bytesRemoved += child.Size()
				continue
			}
		case *mp4.SgpdBox:
			if box.GroupingType == "seam" || box.GroupingType == "seig" {
				bytesRemoved += child.Size()
				continue
			}
		}
		remainingChildren = append(remainingChildren, child)
	}
	return remainingChildren, bytesRemoved
}

func TransformInit(init *mp4.InitSegment) (map[uint32]mp4.DecryptTrackInfo, error) {
	di, err := mp4.DecryptInit(init)
	tracks := make(map[uint32]mp4.DecryptTrackInfo, len(di.TrackInfos))
	for _, ti := range di.TrackInfos {
		tracks[ti.TrackID] = ti
	}
	if err != nil {
		return tracks, err
	}
	for _, trak := range init.Moov.Traks {
		stbl := trak.Mdia.Minf.Stbl
		stbl.Children, _ = FilterSbgpSgpd(stbl.Children)
	}
	return tracks, nil
}

func cbcsDecryptRaw(data []byte, decryptBlockLen, skipBlockLen int, tmpl *template) error {
	if skipBlockLen != 0 {
		return fmt.Errorf("not full encryption of subsamples")
	}
	truncatedLen := len(data) & ^0xf
	decrypted := decryptSample(tmpl, data[:truncatedLen])
	copy(data[:truncatedLen], decrypted)
	return nil
}

func cbcsDecryptSample(sample []byte, subSamplePatterns []mp4.SubSamplePattern, tenc *mp4.TencBox, tmpl *template) error {
	decryptBlockLen := int(tenc.DefaultCryptByteBlock) * 16
	skipBlockLen := int(tenc.DefaultSkipByteBlock) * 16
	var pos uint32 = 0

	if len(subSamplePatterns) == 0 {
		return cbcsDecryptRaw(sample, decryptBlockLen, skipBlockLen, tmpl)
	}

	for j := 0; j < len(subSamplePatterns); j++ {
		ss := subSamplePatterns[j]
		pos += uint32(ss.BytesOfClearData)

		if ss.BytesOfProtectedData <= 0 {
			continue
		}

		err := cbcsDecryptRaw(sample[pos:pos+ss.BytesOfProtectedData], decryptBlockLen, skipBlockLen, tmpl)
		if err != nil {
			return err
		}
		pos += ss.BytesOfProtectedData
	}

	return nil
}

func cbcsDecryptSamples(samples []mp4.FullSample, tmpl *template,
	tenc *mp4.TencBox, senc *mp4.SencBox) error {

	for i := range samples {
		var subSamplePatterns []mp4.SubSamplePattern
		if len(senc.SubSamples) != 0 {
			subSamplePatterns = senc.SubSamples[i]
		}
		err := cbcsDecryptSample(samples[i].Data, subSamplePatterns, tenc, tmpl)
		if err != nil {
			return err
		}
	}
	return nil
}

func DecryptFragment(frag *mp4.Fragment, tracks map[uint32]mp4.DecryptTrackInfo, tmpl *template) error {
	moof := frag.Moof
	var bytesRemoved uint64 = 0

	for _, traf := range moof.Trafs {
		ti, ok := tracks[traf.Tfhd.TrackID]
		if !ok {
			return fmt.Errorf("could not find decryption info for track %d", traf.Tfhd.TrackID)
		}
		if ti.Sinf == nil {
			continue
		}

		schemeType := ti.Sinf.Schm.SchemeType
		if schemeType != "cbcs" {
			return fmt.Errorf("scheme type %s not supported", schemeType)
		}
		hasSenc, isParsed := traf.ContainsSencBox()
		if !hasSenc {
			return fmt.Errorf("no senc box in traf")
		}

		var senc *mp4.SencBox
		if traf.Senc != nil {
			senc = traf.Senc
		} else {
			senc = traf.UUIDSenc.Senc
		}

		if !isParsed {
			err := senc.ParseReadBox(ti.Sinf.Schi.Tenc.DefaultPerSampleIVSize, traf.Saiz)
			if err != nil {
				return err
			}
		}

		samples, err := frag.GetFullSamples(ti.Trex)
		if err != nil {
			return err
		}

		err = cbcsDecryptSamples(samples, tmpl, ti.Sinf.Schi.Tenc, senc)
		if err != nil {
			return err
		}

		bytesRemoved += traf.RemoveEncryptionBoxes()
	}
	_, psshBytesRemoved := moof.RemovePsshs()
	bytesRemoved += psshBytesRemoved
	for _, traf := range moof.Trafs {
		for _, trun := range traf.Truns {
			trun.DataOffset -= int32(bytesRemoved)
		}
	}

	return nil
}
