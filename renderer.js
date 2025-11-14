const taskBar = document.getElementById('task-bar');
const queueButton = document.getElementById('queue-button');
const queueCount = document.getElementById('queue-count');
const minimizeButton = document.getElementById('minimize-button');
const restoreButton = document.getElementById('restore-button');
const backButton = document.getElementById('back-button');
const fwdButton = document.getElementById('fwd-button');
const refreshButton = document.getElementById('refresh-button');
const btnServiceMusic = document.getElementById('btn-service-music');
const btnServiceClassical = document.getElementById('btn-service-classical');
const btnEditConfig = document.getElementById('btn-edit-config');
const urlInput = document.getElementById('url-input');
const urlDownloadButton = document.getElementById('url-download-button');
const display = document.getElementById('current-task-display');
const albumNameEl = document.getElementById('current-album-name');
const albumStatusEl = document.getElementById('album-status-text');
const trackListContainer = document.getElementById('track-list-container');
const cancelBtn = document.getElementById('current-task-cancel-button');

let currentTaskId = null;
let currentAlbumName = "等待任务...";
let taskDataStore = {};
let queueLength = 0;
let clearTaskTimer = null; 
let currentAlbumTotalTracks = 0;

const MUSIC_URL = 'https://music.apple.com/cn/new';
const CLASSICAL_URL = 'https://classical.music.apple.com/cn';

minimizeButton.addEventListener('click', () => {
    taskBar.classList.add('collapsed');
    window.desktopApp.minimizeTaskbar(); 
});
restoreButton.addEventListener('click', () => {
    taskBar.classList.remove('collapsed');
    window.desktopApp.restoreTaskbar(); 
});

backButton.addEventListener('click', () => window.desktopApp.navigateBack());
fwdButton.addEventListener('click', () => window.desktopApp.navigateFwd());
refreshButton.addEventListener('click', () => window.desktopApp.refreshPage());

queueButton.addEventListener('click', () => {
    console.log("show-queue clicked");
    window.desktopApp.showQueue();
});

btnServiceMusic.addEventListener('click', () => {
    window.desktopApp.switchService(MUSIC_URL);
});
btnServiceClassical.addEventListener('click', () => {
    window.desktopApp.switchService(CLASSICAL_URL);
});
btnEditConfig.addEventListener('click', () => {
    window.desktopApp.openConfig();
});

urlDownloadButton.addEventListener('click', () => {
    const url = urlInput.value.trim();
    if (url && window.desktopApp.downloadUrl) {
        window.desktopApp.downloadUrl(url);
        urlInput.value = '';
    }
});
urlInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
        urlDownloadButton.click();
    }
});

window.desktopApp.onSetTaskbarState((isMinimized) => {
    if (isMinimized) {
        taskBar.classList.add('collapsed');
    } else {
        taskBar.classList.remove('collapsed');
    }
});

window.desktopApp.onSetActiveService((url) => {
    if (!btnServiceMusic || !btnServiceClassical) return; 
    btnServiceMusic.classList.toggle('active', url.startsWith(MUSIC_URL));
    btnServiceClassical.classList.toggle('active', url.startsWith(CLASSICAL_URL));
});

window.desktopApp.onSetQueueLength((length) => {
    queueLength = length;
    updateQueueCount(queueLength);
});

window.desktopApp.onTaskQueued(() => {
    queueLength++;
    updateQueueCount(queueLength);
});

window.desktopApp.onTaskCancelled(() => {
    queueLength--;
    if (queueLength < 0) queueLength = 0;
    updateQueueCount(queueLength);
});

window.desktopApp.onTaskStarted((task) => {
    queueLength--;
    if (queueLength < 0) queueLength = 0;
    updateQueueCount(queueLength); 

    if (clearTaskTimer) {
        clearTimeout(clearTaskTimer);
        clearTaskTimer = null;
    }
    
    currentTaskId = task.id;
    currentAlbumName = task.name; 
    taskDataStore = {}; 
    trackListContainer.innerHTML = ''; 
    currentAlbumTotalTracks = task.totalTracks || 0; 
    
    display.classList.remove('idle');
    albumNameEl.textContent = currentAlbumName;
    albumStatusEl.textContent = `( ${task.completedTracks || 0} / ${task.totalTracks || '?'} )`; 
    cancelBtn.style.visibility = 'visible';
    cancelBtn.onclick = () => window.desktopApp.cancelTask(currentTaskId);
});

window.desktopApp.onTaskFinished((taskId) => {
    if (taskId === currentTaskId) {
        const allDone = Object.values(taskDataStore).every(track => 
            track.status === 'complete' || track.status === 'error' || track.status === 'exists'
        );
        
        const allExpectedDone = allDone && (Object.keys(taskDataStore).length === currentAlbumTotalTracks);
        
        if(allExpectedDone) {
            autoClearCurrentTask('已完成', false);
        } else {
            autoClearCurrentTask('任务中断', true);
        }
    }
});

window.desktopApp.onTaskProgressUpdate(({ id, completed, total }) => {
    if (id === currentTaskId) {
        currentAlbumTotalTracks = total;
        albumStatusEl.textContent = `( ${completed} / ${total} )`;
    }
});

window.desktopApp.onGoOutput((message) => {
    let data;
    try { data = JSON.parse(message); } catch (e) { return; }
    if (data.taskId !== currentTaskId) return; 

    if (data.status === 'album-start') {
        console.log("New album signal received, clearing UI for:", data.albumName);
        taskDataStore = {}; 
        trackListContainer.innerHTML = ''; 
        currentAlbumTotalTracks = data.totalTracks || 0; 
        
        albumNameEl.textContent = data.albumName || 'Loading...';
        currentAlbumName = data.albumName || 'Loading...';
        albumStatusEl.textContent = `( 0 / ${currentAlbumTotalTracks || '?'} )`;
        display.classList.remove('idle');
        
        return; 
    }

    if (data.albumName && albumNameEl.textContent !== data.albumName) {
        albumNameEl.textContent = data.albumName;
        currentAlbumName = data.albumName; 
    }

    const trackNum = parseInt(data.trackNum, 10);

    if (typeof data.trackNum === 'undefined' || isNaN(trackNum) || trackNum === 0) {
        if (data.status === 'log') {
            console.log(`[Go Log]: ${data.message}`);
        } else if (data.status === 'error') {
            console.error(`[Go Error]: ${data.message}`);
            albumStatusEl.textContent = `错误: ${data.message.substring(0, 50)}...`;
        }
        return; 
    }

    if (trackNum > currentAlbumTotalTracks) {
        currentAlbumTotalTracks = trackNum;
    }

    const trackNumFormatted = String(data.trackNum).padStart(2, '0');
    const trackName = data.trackName || '未知曲目';
    const fullTrackName = `${trackNumFormatted}. ${trackName}`;

    let trackRow = taskDataStore[data.trackNum];
    
    if (!trackRow) {
        trackRow = document.createElement('div');
        trackRow.className = 'track-row';
        trackRow.innerHTML = `
            <span class="track-label" title="${fullTrackName}">${fullTrackName}</span>
            <div class="task-progress">
                <div class="progress-bar">0%</div>
            </div>
            <span class="track-status-text"></span>
        `;
        
        const initialSortOrder = getSortOrder('start');
        trackRow.dataset.sortOrder = initialSortOrder; 
        
        const firstOrder3Element = trackListContainer.querySelector('[data-sort-order="3"]');
        if (firstOrder3Element) {
            trackListContainer.insertBefore(trackRow, firstOrder3Element);
        } else {
            trackListContainer.appendChild(trackRow); 
        }

        taskDataStore[data.trackNum] = {
            element: trackRow,
            status: 'start',
            sortOrder: initialSortOrder, 
            percentage: 0,
            progressBar: trackRow.querySelector('.progress-bar'),
            statusText: trackRow.querySelector('.track-status-text'),
            trackName: fullTrackName 
        };
    }

    const track = taskDataStore[data.trackNum];

    if (track.trackName !== fullTrackName) {
        const trackLabel = track.element.querySelector('.track-label');
        trackLabel.textContent = fullTrackName;
        trackLabel.title = fullTrackName;
        track.trackName = fullTrackName;
    }

    const newStatus = data.status;
    const newSortOrder = getSortOrder(newStatus);
    const currentSortOrder = track.sortOrder;
    
    if (newSortOrder !== currentSortOrder) {
        track.sortOrder = newSortOrder;
        track.element.dataset.sortOrder = newSortOrder;

        if (newSortOrder === 1) { 
            trackListContainer.prepend(track.element);
        } else if (newSortOrder === 3) { 
            trackListContainer.appendChild(track.element);
        } else { 
            const firstOrder3Element = trackListContainer.querySelector('[data-sort-order="3"]');
            if (firstOrder3Element) {
                trackListContainer.insertBefore(track.element, firstOrder3Element);
            } else {
                trackListContainer.appendChild(track.element);
            }
        }
    }

    switch (newStatus) {
        case 'start':
            track.status = 'progress';
            track.percentage = 0;
            track.statusText.textContent = '等待中...';
            break;
            
        case 'progress':
            track.status = 'progress';
            track.percentage = data.percentage;
            track.statusText.textContent = `下载 ${data.speed}`;
            track.progressBar.style.width = `${data.percentage}%`;
            track.progressBar.textContent = `${data.percentage}%`; 
            track.progressBar.style.backgroundColor = 'var(--color-white)'; 
            track.progressBar.style.color = '#000'; 
            break;
            
        case 'decrypt':
            track.status = 'decrypt';
            track.percentage = data.percentage;
            track.statusText.textContent = `解密 ${data.speed}`;
            track.progressBar.style.width = `${data.percentage}%`;
            track.progressBar.textContent = `${data.percentage}%`; 
            track.progressBar.style.backgroundColor = 'var(--color-bright-yellow)'; 
            track.progressBar.style.color = '#000'; 
            break;
            
        case 'fix':
            track.status = 'fix';
            track.statusText.textContent = '修复中...';
            break;
        case 'exists':
            track.status = 'exists'; 
            track.percentage = 100;
            track.statusText.textContent = '已存在';
            track.progressBar.style.width = '100%';
            track.progressBar.textContent = '100%';
            track.progressBar.style.backgroundColor = 'var(--color-green)'; 
            track.progressBar.style.color = '#000'; 
            break;
        case 'complete':
            track.status = 'complete';
            track.percentage = 100;
            track.statusText.textContent = '完成';
            track.progressBar.style.width = '100%';
            track.progressBar.textContent = '100%';
            track.progressBar.style.backgroundColor = 'var(--color-green)'; 
            track.progressBar.style.color = '#000'; 
            break;
        case 'error':
            track.status = 'error';
            track.percentage = 0;
            track.statusText.textContent = '错误';
            console.error(`[Go Error] ${data.message}`);
            break;
    }
});

function getSortOrder(status) {
    switch (status) {
        case 'decrypt':
            return 1;
        case 'progress':
        case 'fix':
        case 'start':
            return 2;
        case 'complete':
        case 'exists':
        case 'error':
            return 3;
        default:
            return 2;
    }
}

function updateQueueCount(length) {
    const waitingTasks = queueLength;
    queueCount.textContent = waitingTasks;
    queueButton.style.display = waitingTasks > 0 ? 'inline-flex' : 'none';
}

function updateAlbumProgress() {
}

function autoClearCurrentTask(status, isError) {
    currentTaskId = null;
    cancelBtn.style.visibility = 'hidden';
    
    if (isError) {
        albumNameEl.textContent = "任务已取消/失败";
        albumStatusEl.textContent = "";
        display.classList.add('idle');
    } else {
        albumNameEl.textContent = currentAlbumName;
        albumStatusEl.textContent = status;
    }
    
    if (clearTaskTimer) clearTimeout(clearTaskTimer); 
    
    clearTaskTimer = setTimeout(() => {
        display.classList.add('idle');
        albumNameEl.textContent = '等待任务...';
        albumStatusEl.textContent = '';
        trackListContainer.innerHTML = ''; 
        taskDataStore = {};
        currentAlbumTotalTracks = 0; 
        
        clearTaskTimer = null;
    }, 3000); 
}
