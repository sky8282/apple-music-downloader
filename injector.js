(function() {
    'use strict';
    const CONFIG = {
        ALBUM_HEADER_ACTIONS: '.primary-actions',
        ARTIST_HEADER_PLAY_BTN: 'span.artist-header__play-button',
        TRACK_ROW_CONTROLS: '.songs-list-row__controls',
        CARD_ARTWORK: 'div[data-testid="artwork-component"]',
        VIDEO_WRAPPER: 'div[data-testid="vertical-video-artwork-wrapper"]' 
    };
    const MAIN_BTN_CONTAINER_ID = 'custom-main-button-container';
    let currentAlbumTraits = [];
    
    const badgeStyle = document.createElement('style');
    badgeStyle.innerHTML = `
        .custom-button-container { display: inline-flex; align-items: center; gap: 10px; margin-left: 10px; vertical-align: middle; flex-shrink: 0; }
        .custom-dl-btn { border: none !important; border-radius: 50px !important; font-weight: bold !important; cursor: pointer !important; transition: transform 0.2s ease, background-color 0.2s ease !important; line-height: 1.2 !important; display: inline-flex !important; align-items: center; gap: 8px; z-index: 9999; white-space: nowrap; }
        .custom-dl-btn:hover { transform: scale(1.05); } 
        .custom-dl-btn svg { width: 18px; height: 18px; }
        .dl-btn-green { background-color: #1DB954 !important; color: black !important; }
        .dl-btn-green:hover { background-color: #25d865 !important; }
        .dl-btn-green svg { fill: black; }
        .dl-btn-red { background-color: #e74c3c !important; color: white !important; }
        .dl-btn-red:hover { background-color: #f95f51 !important; }
        .dl-btn-red svg { fill: white; }
        .main-dl-btn { padding: 8px 16px !important; font-size: 14px !important; }
        .track-dl-btn { padding: 6px !important; gap: 0 !important; border-radius: 50% !important; margin-left: 0 !important; }
        .track-dl-btn svg { width: 16px; height: 16px; margin: 2px; }
        .track-dl-btn span { display: none; }

        .card-dl-container {
            opacity: 1 !important; 
            display: flex;
            justify-content: center;
            pointer-events: none;
            padding: 8px;
            z-index: 99;
        }
        .card-dl-container .custom-dl-btn {
            pointer-events: auto !important;
            padding: 6px !important; 
            gap: 0 !important; 
            border-radius: 50% !important; 
            margin-left: 0 !important;
            border: 2px solid black !important;
        }
        .card-dl-container .custom-dl-btn svg {
            width: 16px !important; 
            height: 16px !important; 
            margin: 2px !important;
        }
        .card-dl-container .custom-dl-btn span {
            display: none !important;
        }
        
        .ame-track-quality {
            font-size: 10px;
            color: var(--systemSecondary);
            margin-left: 8px;
            line-height: 1.4;
            text-align: left;
            white-space: pre-wrap;
            font-family: monospace;
        }
        .songs-list-row__song-wrapper {
             display: flex;
             align-items: center;
        }

        .navigation-items__header[data-ame]{border-radius:6px;font-size:10px;font-weight:600;letter-spacing:0;line-height:1.3;margin:0 20px -3px;padding:4px 6px;color:var(--systemSecondary)}
        .navigation-items__list[data-ame]{font-size:15px;padding:0 25px 9px;font-weight:400;letter-spacing:0}
        .navigation-item[data-ame]{margin-bottom:1px;height:32px;padding:4px;position:relative;border-radius:6px;--linkHoverTextDecoration: none}
        .navigation-item__link[data-ame]{align-items:center;border-radius:6px;box-sizing:content-box;-moz-column-gap:8px;column-gap:8px;display:flex;height:100%;margin:-3px;padding:3px;width:100%;font-size:.8rem}
        .navigation-item__link[data-ame] svg{width:24px;height:24px;fill:var(--systemSecondary);background-color:transparent;display:inline-block;flex-shrink:0}
        
        .ame-album-badges-container {
            display: flex;
            flex-wrap: wrap;
            gap: 8px;
            margin-top: 1em;
            margin-bottom: 0.5em;
        }
        
        .ame-badge-text {
            display: inline-block;
            font-size: 10px;
            font-weight: 600;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            padding: 3px 7px;
            border-radius: 4px;
            background-color: transparent;
            color: var(--systemSecondary);
            border: 1px solid var(--systemSecondary);
            text-transform: uppercase;
            letter-spacing: 0.5px;
            line-height: 1.2;
        }

        .ame-album-badges-container > svg {
            margin: 0;
            fill: var(--systemSecondary);
            height: 18px; 
        }
    `;
    document.head.appendChild(badgeStyle);

    function kn(t){const e=document.createElement("template");return e.innerHTML=t,e.content.firstElementChild}
    function gl(t,e){return new Promise(n=>{const r=e==null?void 0:e.waitSelector,i=(e==null?void 0:e.timeout)??3e3;if(i!==0){const a=document.querySelector(t);if(a){n(a);return}}const o=setTimeout(()=>{i!==0&&(s.disconnect(),n(null))},i),s=new MutationObserver(a=>{for(const l of a)for(const c of Array.from(l.addedNodes))if(c instanceof Element&&c.matches(r??t)){i!==0&&(s.disconnect(),clearTimeout(o)),n(r?document.querySelector(t):c);return}});s.observe(document.body,{childList:!0,subtree:!0})})}
    function ko(t,e){return kn(`
		<li class="ame-sidebar-button navigation-item" data-ame>
			<a class="navigation-item__link" tabindex="0" data-ame>
				${e}
				<span>${t}</span>
			</a>
		</li>
    `)}
    function e0(t){return t.addMenuItem=(e,n)=>{let r=t;for(let i=0;i<100;i++){const o=r.nextElementSibling;if(!o||Number(o.getAttribute("data-index"))>n)break;r=o}return e.setAttribute("data-index",n.toString()),r.after(e),e},t}
    function tT(t,e){let n=document.querySelector(t);if(n)return e0(n);try{e()}catch{console.error(`Could not create menu reference element for selector "${t}".`)}if(n=document.querySelector(t),n)return e0(n);throw new Error(`Could not find menu reference element by selector "${t}".`)}
    async function Po(t,e){await gl("amp-chrome-player"),tT("#ame-sidebar",()=>{const r=document.querySelector(".navigation__scrollable-container");r==null||r.appendChild(kn(`
			<div class="navigation-items" data-ame>
				<div class="navigation-items__header" data-ame>
					<span>Ame</span>
				</div>
				<ul class="navigation-items__list" data-ame>
					<li id="ame-sidebar" style="display: none;"></li>
				</ul>
			</div>
		`))}).addMenuItem(t,e)}
    
    const Gp="[a-z]{2}/album/(.+/)?.+";
    const Au={};
    function qp(){for(const t of Object.values(Au)){const e=t.pattern.test(location.pathname)?t.onCallbacks:t.offCallbacks;for(const n of e)n()}}
    function Yp(t){const e=new RegExp(`^/${t.replaceAll("/","\\/")}$`);let n=Au[t];return n||(n={pattern:e,onCallbacks:[],offCallbacks:[]},Au[t]=n,n)}
    function ZE(t,e){const n=Yp(t),r=n.pattern.test(location.pathname);n.onCallbacks.push(e),r&&e()}
    function XE(t,e){const n=Yp(t),r=n.pattern.test(location.pathname);n.offCallbacks.push(e),r||e()}
    function ji(t){ZE(Gp,t)}
    function Ps(t){XE(Gp,t)}

    const LE = `<span class="ame-badge-text">AAC</span>`;
    const FE = `<span class="ame-badge-text">Master</span>`;
    const VE = `<span class="ame-badge-text">Atmos</span>`;
    const WE = `<span class="ame-badge-text">Hi-Res</span>`;
    const BE = `<span class="ame-badge-text">Lossless</span>`;
    const jE = `<span class="ame-badge-text">Spatial</span>`;

    const rT=`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" slot="app-icon">
	<path d="M29.75 33.4h2.5v-3.25h2.05q.7 0 1.2-.475T36 28.5v-8.95q0-.7-.5-1.2t-1.2-.5H28q-.7 0-1.35.5-.65.5-.65 1.2v8.95q0 .7.65 1.175.65.475 1.35.475h1.75ZM12 30.15h2.5V25.7h5v4.45H22v-12.3h-2.5v5.35h-5v-5.35H12Zm16.5-2.5v-7.3h5v7.3ZM7 40q-1.2 0-2.1-.9Q4 38.2 4 37V11q0-1.2.9-2.1Q5.8 8 7 8h34q1.2 0 2.1.9.9.9.9 2.1v26q0 1.2-.9 2.1-.9.9-2.1.9Zm0-3h34V11H7v26Zm0 0V11v26Z" />
    </svg>
    `;
    const Hu=ko("Search Covers",`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" slot="app-icon">
	<path d="M24 44q-4.1 0-7.75-1.575-3.65-1.575-6.375-4.3-2.725-2.725-4.3-6.375Q4 28.1 4 24q0-4.25 1.6-7.9 1.6-3.65 4.375-6.35 2.775-2.7 6.5-4.225Q20.2 4 24.45 4q3.95 0 7.5 1.325T38.175 9q2.675 2.35 4.25 5.575Q44 17.8 44 21.65q0 5.4-3.15 8.525T32.5 33.3h-3.75q-.9 0-1.55.7t-.65 1.55q0 1.35.725 2.3.725.95.725 2.2 0 1.9-1.05 2.925T24 44Zm0-20Zm-11.65 1.3q1 0 1.75-.75t.75-1.75q0-1-.75-1.75t-1.75-.75q-1 0-1.75.75t-.75 1.75q0 1 .75 1.75t1.75.75Zm6.3-8.5q1 0 1.75-.75t.75-1.75q0-1-.75-1.75t-1.75-.75q-1 0-1.75.75t-.75 1.75q0 1 .75 1.75t1.75.75Zm10.7 0q1 0 1.75-.75t.75-1.75q0-1-.75-1.75t-1.75-.75q-1 0-1.75.75t-.75 1.75q0 1 .75 1.75t1.75.75Zm6.55 8.5q1 0 1.75-.75t.75-1.75q0-1-.75-1.75t-1.75-.75q-1 0-1.75.75t-.75 1.75q0 1 .75 1.75t1.75.75ZM24 41q.55 0 .775-.225.225-.225.225-.725 0-.7-.725-1.3-.725-.6-.725-2.65 0-2.3 1.5-4.05t3.8-1.75h3.65q3.8 0 6.15-2.225Q41 25.85 41 21.65q0-6.6-5-10.625T24.45 7q-7.3 0-12.375 4.925T7 24q0 7.05 4.975 12.025Q16.95 41 24 41Z" />
    </svg>
    `);
    const Wp=ko("Seed MusicBrainz",`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 -960 960 960" slot="app-icon">
	<path d="M440-120v-319q-64 0-123-24.5T213-533q-45-45-69-104t-24-123v-80h80q63 0 122 24.5T426-746q31 31 51.5 68t31.5 79q5-7 11-13.5t13-13.5q45-45 104-69.5T760-720h80v80q0 64-24.5 123T746-413q-45 45-103.5 69T520-320v200h-80Zm0-400q0-48-18.5-91.5T369-689q-34-34-77.5-52.5T200-760q0 48 18 92t52 78q34 34 78 52t92 18Zm80 120q48 0 91.5-18t77.5-52q34-34 52.5-78t18.5-92q-48 0-92 18.5T590-569q-34 34-52 77.5T520-400Zm0 0Zm-80-120Z" />
    </svg>
    `);
    const ku=ko("检查曲目音质",rT);
    
    function injectSidebarButtons() {
        Hu.addEventListener("click",()=>{const t=document.querySelector(".headings__subtitles > a"),e=document.querySelector(".headings__title");if(!e)return;const n=t==null?void 0:t.innerText.trim(),r=e.innerText.trim().replace(/ - Single$/i,"").replace(/ - EP$/i,""),i=new URLSearchParams;n&&i.set("artist",n),i.set("album",r),open(`https://covers.musichoarders.xyz?${i}`,"_blank")});
        Wp.addEventListener("click",()=>{open(`https://seed.musichoarders.xyz?identifier=${encodeURIComponent(location.href)}`,"_blank")});
        ku.addEventListener("click", () => {
            const trackList = document.querySelector('[data-testid="track-list-item"]');
            if (trackList) {
                const pathParts = window.location.pathname.split('/');
                const albumId = pathParts.pop();
                if (albumId && pathParts.includes('album')) {
                    qualityDataCache = [];
                    document.querySelectorAll('.ame-track-quality').forEach(el => el.remove());
                    window.desktopApp.requestAlbumTracksQuality(albumId); 
                }
            }
        });

        ji(()=>{
            Po(Hu,400);
            Po(Wp,500);
            Po(ku,200);
        });
        Ps(()=>{
            document.querySelectorAll('.ame-sidebar-button').forEach(el => el.remove());
        });
        qp();
    }
    
    function injectAlbumBadges() {
        window.desktopApp.onAlbumInfoResult((data) => {
            if (!data) return;
            
            gl(".headings__metadata-bottom").then(e => {
                if (!e) return;
                
                document.querySelector('.ame-album-badges-container')?.remove();

                const n = data.audioTraits || []; 
                if (data.isMasteredForItunes) { 
                    n.push("adm");
                }
                currentAlbumTraits = [...n]; 
                
                if (n.length === 0) return; 

                const r = kn('<p class="ame-album-badges-container"></p>');
                
                if (n.includes("lossy-stereo")) r.insertAdjacentHTML("beforeend", LE);
                if (n.includes("lossless")) r.insertAdjacentHTML("beforeend", BE);
                if (n.includes("hi-res-lossless")) r.insertAdjacentHTML("beforeend", WE);
                if (n.includes("atmos")) r.insertAdjacentHTML("beforeend", VE);
                if (n.includes("adm")) r.insertAdjacentHTML("beforeend", FE);
                if (n.includes("spatial")) r.insertAdjacentHTML("beforeend", jE);
                
                e.after(r);

                if (currentAlbumTraits.includes("atmos")) {
                    const btnContainer = document.getElementById(MAIN_BTN_CONTAINER_ID);
                    if (btnContainer && !btnContainer.querySelector('.dl-btn-red')) {
                        const details = { name: document.querySelector('h1')?.textContent.trim() || document.title, artist: document.querySelector('.product-header__identity a')?.textContent.trim() || '未知歌手' };
                        const url = new URL(window.location.href).href;
                        
                        const atmosButtonEl = createButtons(url, details, false, 'atmos');
                        atmosButtonEl.classList.remove('dl-btn-green');
                        atmosButtonEl.classList.add('dl-btn-red');
                        const span = atmosButtonEl.querySelector('span');
                        if(span) span.textContent = 'Atmos';
                        atmosButtonEl.title = "下载 (Atmos)";
                        
                        btnContainer.appendChild(atmosButtonEl);
                    }
                }
            });
        });

        ji(() => {
            const pathParts = window.location.pathname.split('/');
            const albumId = pathParts.pop();
            
            if (albumId && pathParts.includes('album')) {
                document.querySelector('.ame-album-badges-container')?.remove();
                currentAlbumTraits = [];
                window.desktopApp.requestAlbumInfo(albumId);
            }
        });

        Ps(() => {
            document.querySelector('.ame-album-badges-container')?.remove();
            currentAlbumTraits = [];
        });
    }

    const downloadIconSVG = `<svg viewBox="0 0 24 24"><path d="M19.35 10.04C18.67 6.59 15.64 4 12 4 9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96zM17 13l-5 5-5-5h3V9h4v4h3z"></path></svg>`;

    function createButtons(url, details, isSmall = false, downloadType = 'normal') {
        const btnSizeClass = isSmall ? 'track-dl-btn' : 'main-dl-btn';
        const textSpan = isSmall ? '' : '<span>下载</span>';
        const button = document.createElement('button');
        button.className = `custom-dl-btn dl-btn-green ${btnSizeClass}`;
        button.title = "下载";
        button.innerHTML = `${downloadIconSVG} ${textSpan}`;
        const stopAllClicks = (e) => {
            e.preventDefault();
            e.stopPropagation();
            e.stopImmediatePropagation();
        };
        button.addEventListener('pointerdown', (e) => {
            stopAllClicks(e);
            if (window.desktopApp && typeof window.desktopApp.download === 'function') {
                window.desktopApp.download(url, details, downloadType); 
            } else { 
                console.error('(preload.js) 未找到'); 
            }
        }, { capture: true });
        button.addEventListener('pointerup', stopAllClicks, { capture: true });
        button.addEventListener('click', stopAllClicks, { capture: true });
        return button;
    }

    function injectAlbumHeaderButton(container) {
        if (container.querySelector(`#${MAIN_BTN_CONTAINER_ID}`)) return;
        const details = { name: document.querySelector('h1')?.textContent.trim() || document.title, artist: document.querySelector('.product-header__identity a')?.textContent.trim() || '未知歌手' };
        const url = new URL(window.location.href).href;
        
        const buttonContainer = document.createElement('div'); 
        buttonContainer.className = 'custom-button-container'; 
        buttonContainer.id = MAIN_BTN_CONTAINER_ID; 

        const buttonEl = createButtons(url, details, false, 'normal');
        buttonContainer.appendChild(buttonEl); 
        
        container.appendChild(buttonContainer); 
        container.classList.add('custom-buttons-added');
    }
    function injectArtistHeaderButton(playButtonSpan) {
        const nameEl = document.querySelector('h1[data-testid="artist-header-name"]'); const name = nameEl?.textContent.trim(); if (!name) return;
        const details = { name: name, artist: name };     
        const url = new URL(window.location.href).href;
        const buttonEl = createButtons(url, details, false, 'all-album');
        const buttonContainer = document.createElement('div'); buttonContainer.className = 'custom-button-container'; buttonContainer.id = MAIN_BTN_CONTAINER_ID; buttonContainer.appendChild(buttonEl);
        playButtonSpan.parentNode.insertBefore(buttonContainer, playButtonSpan.nextSibling); playButtonSpan.classList.add('custom-buttons-added');
    }

    function injectTrackButton(row) {
        const controlsContainer = row.querySelector(CONFIG.TRACK_ROW_CONTROLS); 
        if (!controlsContainer) return;
        const allLinks = Array.from(row.querySelectorAll('a[data-testid="click-action"]'));
        const artistLinks = Array.from(row.querySelectorAll('[data-testid="track-title-by-line"] a'));
        const songLink = allLinks.find(link => !artistLinks.includes(link));
        if (!songLink) {
            return;
        }
        const url = new URL(songLink.href, window.location.origin).href;
        const trackTitleEl = row.querySelector('[data-testid="track-title"]');
        const trackArtistEl = row.querySelector('[data-testid="track-title-by-line"]');
        const trackName = trackTitleEl ? trackTitleEl.textContent.trim() : '未知曲目';
        const trackArtist = trackArtistEl ? trackArtistEl.textContent.trim() : '未知歌手';
        const details = { name: trackName, artist: trackArtist };
        const buttonEl = createButtons(url, details, true, 'normal');        
        const buttonContainer = document.createElement('div'); 
        buttonContainer.className = 'custom-button-container'; 
        buttonContainer.appendChild(buttonEl);        
        controlsContainer.appendChild(buttonContainer); 
        row.classList.add('custom-buttons-added');
    }

    function injectCardButton(artworkElement) {
        const cardRoot = artworkElement.closest('li[class*="grid-item"], div[class*="product-card"], div[class*="product-lockup"]');
        if (!cardRoot) return;
        if (artworkElement.classList.contains('custom-buttons-added')) return;
        let link = cardRoot.querySelector('a[data-testid="product-lockup-link"]') || cardRoot.querySelector('div[class*="lockup__title"] > a');
        if (!link || !link.href) return;
        const url = new URL(link.href, window.location.origin).href;
        const name = link.textContent.trim();
        let artist = '未知歌手';
        const artistEl = cardRoot.querySelector('div[class*="lockup__subtitle"]');
        if (artistEl) artist = artistEl.textContent.trim();
        const details = { name, artist };
        const buttonEl = createButtons(url, details, true, 'normal');
        const bottomContainer = document.createElement('div');
        bottomContainer.className = 'card-dl-container';
        bottomContainer.style.position = 'absolute';
        bottomContainer.style.bottom = '0';
        bottomContainer.style.left = '50%';
        bottomContainer.style.transform = 'translateX(-50%)';
        bottomContainer.style.pointerEvents = 'auto';
        bottomContainer.style.zIndex = '100';
        bottomContainer.appendChild(buttonEl);
        cardRoot.style.position = 'relative';
        cardRoot.appendChild(bottomContainer);
        artworkElement.classList.add('custom-buttons-added');
    }

    function injectVideoButton(videoWrapper) {
        if (videoWrapper.classList.contains('custom-buttons-added')) return;
        const linkEl = videoWrapper.querySelector('a[data-testid="click-action"]');
        const artworkEl = videoWrapper.querySelector('[data-testid="artwork-component"]');
        if (!linkEl || !linkEl.href || !artworkEl) return;
        
        const urlObj = new URL(linkEl.href, window.location.origin);
        const url = urlObj.href;

        const cardRoot = videoWrapper.closest('li[class*="grid-item"], div[class*="product-card"], div[class*="product-lockup"]');
        let name = '未知视频';
        let artist = '未知歌手';

        if (cardRoot) {
            const nameEl = cardRoot.querySelector('div[class*="lockup__title"]') || cardRoot.querySelector('a[data-testid="product-lockup-link"]');
            const artistEl = cardRoot.querySelector('div[class*="lockup__subtitle"]');
            if (nameEl) name = nameEl.textContent.trim();
            if (artistEl) artist = artistEl.textContent.trim();
        }

        if (name === '未知视频' || (cardRoot && !cardRoot.querySelector('div[class*="lockup__title"]'))) {
            const pathParts = urlObj.pathname.split('/');
            const videoIndex = pathParts.indexOf('music-video');
            if (videoIndex > -1 && videoIndex + 1 < pathParts.length) {
                const potentialSlug = pathParts[videoIndex + 1];
                if (isNaN(Number(potentialSlug)) && potentialSlug.trim() !== '') {
                    name = potentialSlug;
                }
            }
        }
        
        if (name === '未知视频') {
             name = url.split('/').pop() || '未知视频';
        }

        const details = { name: name, artist: artist };
        const buttonEl = createButtons(url, details, true, 'normal');
        const buttonContainer = document.createElement('div');
        buttonContainer.className = 'card-dl-container';
        buttonContainer.style.position = 'absolute';
        buttonContainer.style.bottom = '0';
        buttonContainer.style.left = '50%';
        buttonContainer.style.transform = 'translateX(-50%)';
        buttonContainer.style.pointerEvents = 'auto';
        buttonContainer.style.zIndex = '100';
        buttonContainer.appendChild(buttonEl);
        videoWrapper.style.position = 'relative';
        videoWrapper.appendChild(buttonContainer);
        videoWrapper.classList.add('custom-buttons-added');
        if (artworkEl) artworkEl.classList.add('custom-buttons-added');
    }

    let qualityDataCache = [];
    
    window.desktopApp.onAlbumQualityResult((qualities) => { 
        qualityDataCache = qualities;
        const trackRows = document.querySelectorAll('.songs-list-row__song-wrapper');
        trackRows.forEach((wrapper, index) => {
            if (qualities[index] && !wrapper.querySelector('.ame-track-quality')) {
                const qualityEl = document.createElement('span');
                qualityEl.className = 'ame-track-quality';
                qualityEl.innerHTML = qualities[index];
                wrapper.appendChild(qualityEl);
            }
        });
    });

    let sidebarInjected = false;

    const observer = new MutationObserver(() => {
        try {
            if (!document.getElementById(MAIN_BTN_CONTAINER_ID)) {
                const albumHeader = document.querySelector(CONFIG.ALBUM_HEADER_ACTIONS + ':not(.custom-buttons-added)');
                if (albumHeader) injectAlbumHeaderButton(albumHeader);
                const artistHeaderBtn = document.querySelector(CONFIG.ARTIST_HEADER_PLAY_BTN + ':not(.custom-buttons-added)');
                if (artistHeaderBtn) injectArtistHeaderButton(artistHeaderBtn);
            }
            
            const trackRows = document.querySelectorAll('[data-testid="track-list-item"]:not(.custom-buttons-added)');
            trackRows.forEach(injectTrackButton);

            const cardArtworkSelector = CONFIG.CARD_ARTWORK + ':not(.custom-buttons-added)';
            document.querySelectorAll(cardArtworkSelector).forEach(injectCardButton);
             document.querySelectorAll(CONFIG.VIDEO_WRAPPER + ':not(.custom-buttons-added)').forEach(injectVideoButton);

            if (!sidebarInjected && document.querySelector(".navigation__scrollable-container")) {
                 injectSidebarButtons();
                 injectAlbumBadges(); 
                 sidebarInjected = true;
            }
            
            const trackList = document.querySelector('[data-testid="track-list-item"]');
            if (trackList && qualityDataCache.length > 0) {
                 const trackWrappers = document.querySelectorAll('.songs-list-row__song-wrapper');
                 trackWrappers.forEach((wrapper, index) => {
                    if (qualityDataCache[index] && !wrapper.querySelector('.ame-track-quality')) {
                        const qualityEl = document.createElement('span');
                        qualityEl.className = 'ame-track-quality';
                        qualityEl.innerHTML = qualityDataCache[index];
                        wrapper.appendChild(qualityEl);
                    }
                });
            }
            
        } catch (err) {
            console.error('[Injector.js] 注入时发生错误:', err);
        }
    });    
    observer.observe(document.body, { childList: true, subtree: true });

    let oldPath = location.pathname;
    const pathObserver = new MutationObserver(() => {
        if (oldPath !== location.pathname) {
            oldPath = location.pathname;
            sidebarInjected = false;
            qualityDataCache = [];
            currentAlbumTraits = [];
            qp();
        }
    });
    pathObserver.observe(document.body, { childList: true, subtree: true });

    function injectNavControls() {
        if (document.getElementById('custom-nav-container')) return;

        const searchWrapper = document.querySelector('[data-testid="search-input"]');

        if (!searchWrapper) return; 

        const navContainer = document.createElement('div');
        navContainer.id = 'custom-nav-container';
        navContainer.style.cssText = `
            display: flex;
            width: 100%;
            box-sizing: border-box;
            align-items: center;
            justify-content: flex-start;
            gap: 8px;
            padding: 0 0 8px 20px;
            -webkit-app-region: no-drag;
        `;

        const btnStyle = `
            background: transparent;
            border: none;
            color: #aaa;
            cursor: pointer;
            padding: 4px;
            border-radius: 4px;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: color 0.2s, background-color 0.2s;
        `;

        const iconBack = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 18l-6-6 6-6"/></svg>`;
        const iconFwd = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6"/></svg>`;
        const iconRefresh = `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M23 4v6h-6"></path><path d="M1 20v-6h6"></path><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>`;

        function createNavBtn(html, title, onClick) {
            const btn = document.createElement('button');
            btn.innerHTML = html;
            btn.title = title;
            btn.style.cssText = btnStyle;
            btn.onmouseenter = () => { btn.style.color = '#fff'; btn.style.backgroundColor = 'rgba(255,255,255,0.1)'; };
            btn.onmouseleave = () => { btn.style.color = '#aaa'; btn.style.backgroundColor = 'transparent'; };
            btn.onclick = (e) => {
                e.preventDefault();
                onClick();
            };
            return btn;
        }

        const btnBack = createNavBtn(iconBack, '后退', () => window.desktopApp.navigateBack());
        const btnFwd = createNavBtn(iconFwd, '前进', () => window.desktopApp.navigateFwd());
        const btnRefresh = createNavBtn(iconRefresh, '刷新', () => window.desktopApp.refreshPage());

        navContainer.appendChild(btnBack);
        navContainer.appendChild(btnFwd);
        navContainer.appendChild(btnRefresh);
        searchWrapper.parentElement.insertBefore(navContainer, searchWrapper);
    }
    setInterval(injectNavControls, 1000);

    console.log('Apple Music 助手已加载');
})();
