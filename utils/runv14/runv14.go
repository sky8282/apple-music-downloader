package runv14

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"main/utils/structs"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/fatih/color"
	"github.com/grafov/m3u8"
)

const prefetchKey = "skd://itunes.apple.com/P000000000/s1/e1"

type ProgressUpdate struct {
	Percentage int
	SpeedBPS   float64
	Stage      string
}

func getRemoteFileSize(fileUrl string, header http.Header) (int64, error) {
	req, err := http.NewRequest("HEAD", fileUrl, nil)
	if err != nil {
		return 0, err
	}
	req.Header = header.Clone()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("server returned status %s", resp.Status)
	}
	size := resp.ContentLength
	if size <= 0 {
		return 0, errors.New("could not determine file size")
	}
	return size, nil
}
func downloadChunk(wg *sync.WaitGroup, errChan chan error, progressBytes chan int64, fileUrl string, header http.Header, tempFile *os.File, chunkIndex int, start, end int64, Config structs.ConfigSet) {
	defer wg.Done()

	req, err := http.NewRequest("GET", fileUrl, nil)
	if err != nil {
		errChan <- fmt.Errorf("chunk %d: failed to create request: %w", chunkIndex, err)
		return
	}

	req.Header = header.Clone()
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errChan <- fmt.Errorf("chunk %d: request failed: %w", chunkIndex, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {

		if !(chunkIndex > 0 && resp.StatusCode == http.StatusOK) {
			errChan <- fmt.Errorf("chunk %d: server returned non-206 status: %s", chunkIndex, resp.Status)
			return
		}
	}
	
	buffer := make([]byte, Config.NetworkReadBufferKB*1024)
	var writtenBytes int64 = 0
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := tempFile.WriteAt(buffer[:n], start+writtenBytes)
			if writeErr != nil {
				errChan <- fmt.Errorf("chunk %d: failed to write to temp file: %w", chunkIndex, writeErr)
				return
			}
			writtenBytes += int64(n)
			progressBytes <- int64(n)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			errChan <- fmt.Errorf("chunk %d: failed to read body stream: %w", chunkIndex, readErr)
			return
		}
	}
}
func downloadFileInChunks(fileUrl string, header http.Header, totalSize int64, numChunks int, progressChan chan ProgressUpdate, Config structs.ConfigSet) (*os.File, error) {
	tempFile, err := os.CreateTemp("", "amdl-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	chunkSize := totalSize / int64(numChunks)
	var wg sync.WaitGroup
	errChan := make(chan error, numChunks)
	progressBytes := make(chan int64, numChunks*10)

	go func() {
		var totalDownloadedBytes int64
		var lastReportedBytes int64
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case bytes, ok := <-progressBytes:
				if !ok { // Channel closed
					progressChan <- ProgressUpdate{Percentage: 100, SpeedBPS: 0, Stage: "download"}
					return
				}
				totalDownloadedBytes += bytes
			case <-ticker.C:
				speed := float64(totalDownloadedBytes-lastReportedBytes) / 0.5
				lastReportedBytes = totalDownloadedBytes

				percentage := int(float64(totalDownloadedBytes) * 100 / float64(totalSize))
				if percentage > 100 {
					percentage = 100
				}
				progressChan <- ProgressUpdate{Percentage: percentage, SpeedBPS: speed, Stage: "download"}
			}
		}
	}()

	for i := 0; i < numChunks; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == numChunks-1 {
			end = totalSize - 1
		}
		wg.Add(1)
		go downloadChunk(&wg, errChan, progressBytes, fileUrl, header, tempFile, i, start, end, Config)
	}

	wg.Wait()
	close(errChan)
	close(progressBytes)

	for err := range errChan {
		if err != nil {
			os.Remove(tempFile.Name())
			return nil, err
		}
	}

	return tempFile, nil
}

func Run(adamId string, playlistUrl string, outfile string, account *structs.Account, Config structs.ConfigSet, progressChan chan ProgressUpdate) error {
	header := make(http.Header)

	req, err := http.NewRequest("GET", playlistUrl, nil)
	if err != nil {
		return err
	}
	req.Header = header
	do, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}

	segments, err := parseMediaPlaylist(do.Body)
	if err != nil {
		return err
	}
	do.Body.Close()

	if len(segments) == 0 || segments[0] == nil {
		return errors.New("no segments extracted from playlist")
	}
	if segments[0].Limit <= 0 {
		return errors.New("non-byterange playlists are currently unsupported")
	}

	parsedUrl, err := url.Parse(playlistUrl)
	if err != nil {
		return err
	}
	fileUrl, err := parsedUrl.Parse(segments[0].URI)
	if err != nil {
		return err
	}
	fileUrlStr := fileUrl.String()

	totalSize, err := getRemoteFileSize(fileUrlStr, header)
	if err != nil {
		return fmt.Errorf("could not get file size: %w", err)
	}

	numChunks := Config.ChunkDownloadThreads
	if numChunks <= 0 {
		numChunks = 10
	}
	tempFile, err := downloadFileInChunks(fileUrlStr, header, totalSize, numChunks, progressChan, Config)
	if err != nil {
		return fmt.Errorf("failed to download file in chunks: %w", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	readTempFile, err := os.Open(tempFile.Name())
	if err != nil {
		return fmt.Errorf("failed to open temp file for reading: %w", err)
	}
	defer readTempFile.Close()

	addr := account.DecryptM3u8Port
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer Close(conn)
	err = downloadAndDecryptFile(conn, readTempFile, totalSize, outfile, adamId, segments, Config, progressChan)
	if err != nil {
		return err
	}

	return nil
}

func downloadAndDecryptFile(conn net.Conn, in io.Reader, totalSize int64, outfile string,
	adamId string, playlistSegments []*m3u8.MediaSegment, Config structs.ConfigSet, progressChan chan ProgressUpdate) error {

	bufferSize := Config.BufferSizeKB * 1024

	ofh, err := os.Create(outfile)
	if err != nil {
		return err
	}
	defer ofh.Close()
	outBuf := bufio.NewWriterSize(ofh, bufferSize)
	inBuf := bufio.NewReaderSize(in, bufferSize)

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

	rw := bufio.NewReadWriter(bufio.NewReaderSize(conn, bufferSize), bufio.NewWriterSize(conn, bufferSize))

	var lastReportedOffset uint64
	lastReportTime := time.Now()

	for i := 0; ; i++ {
		if totalSize > 0 && time.Since(lastReportTime) > 500*time.Millisecond {
			elapsedSeconds := time.Since(lastReportTime).Seconds()
			speed := float64(offset-lastReportedOffset) / elapsedSeconds
			lastReportedOffset = offset
			lastReportTime = time.Now()
			percentage := int(float64(offset) * 100 / float64(totalSize))
			if percentage > 100 {
				percentage = 100
			}
			progressChan <- ProgressUpdate{Percentage: percentage, SpeedBPS: speed, Stage: "decrypt"}
		}

		var frag *mp4.Fragment
		frag, offset, err = ReadNextFragment(inBuf, offset)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if frag == nil {
			break
		}
		if i >= len(playlistSegments) {
			return errors.New("ran out of playlist segments, but more fragments found in file")
		}
		segment := playlistSegments[i]
		if segment == nil {
			return errors.New("segment number out of sync")
		}
		key := segment.Key
		if key != nil {
			if i != 0 {
				SwitchKeys(rw)
			}
			if key.URI == prefetchKey {
				SendString(rw, "0")
			} else {
				SendString(rw, adamId)
			}
			SendString(rw, key.URI)
		}
		err = DecryptFragment(frag, tracks, rw)
		if err != nil {
			return fmt.Errorf("decryptFragment: %w", err)
		}
		err = frag.Encode(outBuf)
		if err != nil {
			return err
		}
	}

	progressChan <- ProgressUpdate{Percentage: 100, SpeedBPS: 0, Stage: "decrypt"}
	err = outBuf.Flush()
	if err != nil {
		return err
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
			return nil, offset, err
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

func Close(conn io.WriteCloser) error {
	defer conn.Close()
	_, err := conn.Write([]byte{0, 0, 0, 0, 0})
	return err
}

func SwitchKeys(conn io.Writer) error {
	_, err := conn.Write([]byte{0, 0, 0, 0})
	return err
}

func SendString(conn io.Writer, uri string) error {
	_, err := conn.Write([]byte{byte(len(uri))})
	if err != nil {
		return err
	}
	_, err = io.WriteString(conn, uri)
	return err
}

func cbcsFullSubsampleDecrypt(data []byte, conn *bufio.ReadWriter) error {
	truncatedLen := len(data) & ^0xf
	if truncatedLen == 0 {
		return nil
	}
	err := binary.Write(conn, binary.LittleEndian, uint32(truncatedLen))
	if err != nil {
		return err
	}
	_, err = conn.Write(data[:truncatedLen])
	if err != nil {
		return err
	}
	err = conn.Flush()
	if err != nil {
		return err
	}
	_, err = io.ReadFull(conn, data[:truncatedLen])
	return err
}

func cbcsStripeDecrypt(data []byte, conn *bufio.ReadWriter, decryptBlockLen, skipBlockLen int) error {
	size := len(data)
	if size < decryptBlockLen {
		return nil
	}
	count := ((size - decryptBlockLen) / (decryptBlockLen + skipBlockLen)) + 1
	totalLen := count * decryptBlockLen

	err := binary.Write(conn, binary.LittleEndian, uint32(totalLen))
	if err != nil {
		return err
	}

	pos := 0
	for {
		if size-pos < decryptBlockLen {
			break
		}
		_, err = conn.Write(data[pos : pos+decryptBlockLen])
		if err != nil {
			return err
		}
		pos += decryptBlockLen
		if size-pos < skipBlockLen {
			break
		}
		pos += skipBlockLen
	}
	err = conn.Flush()
	if err != nil {
		return err
	}

	pos = 0
	for {
		if size-pos < decryptBlockLen {
			break
		}
		_, err = io.ReadFull(conn, data[pos:pos+decryptBlockLen])
		if err != nil {
			return err
		}
		pos += decryptBlockLen
		if size-pos < skipBlockLen {
			break
		}
		pos += skipBlockLen
	}
	return nil
}

func cbcsDecryptRaw(data []byte, conn *bufio.ReadWriter, decryptBlockLen, skipBlockLen int) error {
	if skipBlockLen == 0 {
		return cbcsFullSubsampleDecrypt(data, conn)
	} else {
		return cbcsStripeDecrypt(data, conn, decryptBlockLen, skipBlockLen)
	}
}

func cbcsDecryptSample(sample []byte, conn *bufio.ReadWriter,
	subSamplePatterns []mp4.SubSamplePattern, tenc *mp4.TencBox) error {

	decryptBlockLen := int(tenc.DefaultCryptByteBlock) * 16
	skipBlockLen := int(tenc.DefaultSkipByteBlock) * 16
	var pos uint32 = 0

	if len(subSamplePatterns) == 0 {
		return cbcsDecryptRaw(sample, conn, decryptBlockLen, skipBlockLen)
	}

	for j := 0; j < len(subSamplePatterns); j++ {
		ss := subSamplePatterns[j]
		pos += uint32(ss.BytesOfClearData)

		if ss.BytesOfProtectedData <= 0 {
			continue
		}

		err := cbcsDecryptRaw(sample[pos:pos+ss.BytesOfProtectedData],
			conn, decryptBlockLen, skipBlockLen)
		if err != nil {
			return err
		}
		pos += ss.BytesOfProtectedData
	}

	return nil
}

func cbcsDecryptSamples(samples []mp4.FullSample, conn *bufio.ReadWriter,
	tenc *mp4.TencBox, senc *mp4.SencBox) error {

	for i := range samples {
		var subSamplePatterns []mp4.SubSamplePattern
		if len(senc.SubSamples) != 0 {
			subSamplePatterns = senc.SubSamples[i]
		}
		err := cbcsDecryptSample(samples[i].Data, conn, subSamplePatterns, tenc)
		if err != nil {
			return err
		}
	}
	return nil
}

func DecryptFragment(frag *mp4.Fragment, tracks map[uint32]mp4.DecryptTrackInfo, conn *bufio.ReadWriter) error {
	moof := frag.Moof
	var bytesRemoved uint64 = 0
	var sxxxBytesRemoved uint64

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

		err = cbcsDecryptSamples(samples, conn, ti.Sinf.Schi.Tenc, senc)
		if err != nil {
			return err
		}

		bytesRemoved += traf.RemoveEncryptionBoxes()
		traf.Children, sxxxBytesRemoved = FilterSbgpSgpd(traf.Children)
		bytesRemoved += sxxxBytesRemoved
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

func RunOrchestrated(adamId string, playlistUrl string, targetStorefront string, outfile string, allAccounts []structs.Account, config structs.ConfigSet) error {
	yellow := color.New(color.FgYellow).SprintFunc()

	if targetStorefront != "" {
		fmt.Printf("从链接中识别到区域 (Storefront): %s\n", yellow(strings.ToUpper(targetStorefront)))
	} else {
		fmt.Println("警告: 无法从URL中自动识别区域，将按配置文件顺序尝试所有可用服务。")
	}

	var preferredAccounts []*structs.Account
	var fallbackAccounts []*structs.Account

	for i := range allAccounts {
		acc := &allAccounts[i]
		if targetStorefront != "" && strings.EqualFold(acc.Storefront, targetStorefront) {
			preferredAccounts = append(preferredAccounts, acc)
		} else {
			fallbackAccounts = append(fallbackAccounts, acc)
		}
	}

	orderedAccounts := append(preferredAccounts, fallbackAccounts...)

	if len(orderedAccounts) == 0 {
		return errors.New("配置文件中没有可用的服务 (Account)")
	}

	var lastError error

	for _, acc := range orderedAccounts {
		fmt.Printf("--------------------------------------------------\n")
		fmt.Printf("正在尝试服务: %s (端口: %s, 区域: %s)\n", acc.Name, acc.DecryptM3u8Port, yellow(strings.ToUpper(acc.Storefront)))
		err := Run(adamId, playlistUrl, outfile, acc, config, nil)
		if err == nil {
			fmt.Printf("服务 %s 操作成功！任务完成。\n", acc.Name)
			return nil
		}
		fmt.Printf("警告: 服务 %s 操作失败: %v\n", acc.Name, err)
		lastError = err
	}

	fmt.Println("##################################################")
	fmt.Println("所有可用的服务均尝试失败。")
	fmt.Println("##################################################")
	return fmt.Errorf("所有服务均操作失败，最后一次的错误为: %w", lastError)
}

