// injector.js
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
    const style = document.createElement('style');
    style.innerHTML = `
        .custom-button-container { display: inline-flex; align-items: center; gap: 10px; margin-left: 10px; vertical-align: middle; flex-shrink: 0; }
        .custom-dl-btn { border: none !important; border-radius: 50px !important; font-weight: bold !important; cursor: pointer !important; transition: transform 0.2s ease, background-color 0.2s ease !important; line-height: 1.2 !important; display: inline-flex !important; align-items: center; gap: 8px; z-index: 9999; white-space: nowrap; }
        .custom-dl-btn:hover { transform: scale(1.05); } 
        .custom-dl-btn svg { width: 18px; height: 18px; }
        .dl-btn-green { background-color: #1DB954 !important; color: black !important; }
        .dl-btn-green:hover { background-color: #25d865 !important; }
        .dl-btn-green svg { fill: black; }
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
    `;
    document.head.appendChild(style);

    const downloadIconSVG = `<svg viewBox="0 0 24 24"><path d="M19.35 10.04C18.67 6.59 15.64 4 12 4 9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96zM17 13l-5 5-5-5h3V9h4v4h3z"></path></svg>`;

    function createButtons(url, details, isSmall = false) {
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
                window.desktopApp.download(url, details);
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
        const buttonEl = createButtons(url, details, false);
        const buttonContainer = document.createElement('div'); buttonContainer.className = 'custom-button-container'; buttonContainer.id = MAIN_BTN_CONTAINER_ID; buttonContainer.appendChild(buttonEl);
        container.appendChild(buttonContainer); container.classList.add('custom-buttons-added');
    }
    function injectArtistHeaderButton(playButtonSpan) {
        const nameEl = document.querySelector('h1[data-testid="artist-header-name"]'); const name = nameEl?.textContent.trim(); if (!name) return;
        const details = { name: name, artist: name };     
        const url = new URL(window.location.href).href;
        const buttonEl = createButtons(url, details, false);
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
        const buttonEl = createButtons(url, details, true);        
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
        const buttonEl = createButtons(url, details, true);
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
        const url = new URL(linkEl.href, window.location.origin).href;
        const cardRoot = videoWrapper.closest('li[class*="grid-item"], div[class*="product-card"], div[class*="product-lockup"]');
        let name = '未知视频';
        let artist = '未知歌手';
        if (cardRoot) {
            const nameEl = cardRoot.querySelector('div[class*="lockup__title"]') || cardRoot.querySelector('a[data-testid="product-lockup-link"]');
            const artistEl = cardRoot.querySelector('div[class*="lockup__subtitle"]');
            if (nameEl) name = nameEl.textContent.trim();
            if (name === '未知视频') name = url.split('/')[4] || '未知视频';
            if (artistEl) artist = artistEl.textContent.trim();
        } else {
             name = url.split('/')[4] || '未知视频';
        }
        const details = { name: name, artist: artist };
        const buttonEl = createButtons(url, details, true);
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

    const observer = new MutationObserver(() => {
        try {
            if (!document.getElementById(MAIN_BTN_CONTAINER_ID)) {
                const albumHeader = document.querySelector(CONFIG.ALBUM_HEADER_ACTIONS + ':not(.custom-buttons-added)');
                if (albumHeader) injectAlbumHeaderButton(albumHeader);
                const artistHeaderBtn = document.querySelector(CONFIG.ARTIST_HEADER_PLAY_BTN + ':not(.custom-buttons-added)');
                if (artistHeaderBtn) injectArtistHeaderButton(artistHeaderBtn);
            }
            document.querySelectorAll('[data-testid="track-list-item"]:not(.custom-buttons-added)').forEach(injectTrackButton);
            const cardArtworkSelector = CONFIG.CARD_ARTWORK + ':not(.custom-buttons-added)';
            document.querySelectorAll(cardArtworkSelector).forEach(injectCardButton);
             document.querySelectorAll(CONFIG.VIDEO_WRAPPER + ':not(.custom-buttons-added)').forEach(injectVideoButton);
            
        } catch (err) {
            console.error('[Injector.js] 注入时发生错误:', err);
        }
    });    
    observer.observe(document.body, { childList: true, subtree: true });
    console.log('[Injector.js] 音乐下载助手 (v34 - 视频和卡片按钮修复) 已加载。');
})();