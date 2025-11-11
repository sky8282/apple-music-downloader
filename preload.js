// preload.js
const { contextBridge, ipcRenderer } = require('electron');
contextBridge.exposeInMainWorld('desktopApp', {
    download: (url, details) => {
        ipcRenderer.send('start-download', { url, details });
    },
    downloadUrl: (url) => {
        ipcRenderer.send('start-download-url', url);
    },
    onGoOutput: (callback) => ipcRenderer.on('go-output', (e, msg) => callback(msg)),
    onTaskQueued: (callback) => ipcRenderer.on('task-queued', (e, task) => callback(task)),
    onTaskCancelled: (callback) => ipcRenderer.on('task-cancelled', (e, id) => callback(id)),
    onTaskStarted: (callback) => ipcRenderer.on('task-started', (e, task) => callback(task)),
    onTaskFinished: (callback) => ipcRenderer.on('task-finished', (e, id) => callback(id)),
    onQueueSnapshot: (callback) => ipcRenderer.on('queue-snapshot', (e, queue) => callback(queue)),
    onQueueUpdate: (callback) => ipcRenderer.on('queue-update', (e, tasks) => callback(tasks)),
    onTaskProgressUpdate: (callback) => ipcRenderer.on('task-progress-update', (e, data) => callback(data)),
    onSetTaskbarState: (callback) => ipcRenderer.on('set-taskbar-state', (e, isMinimized) => callback(isMinimized)),
    onSetActiveService: (callback) => ipcRenderer.on('set-active-service', (e, url) => callback(url)),
    onSetQueueLength: (callback) => ipcRenderer.on('set-queue-length', (e, len) => callback(len)),
    switchService: (url) => ipcRenderer.send('switch-service', url),
    cancelTask: (taskId) => ipcRenderer.send('cancel-task', taskId),
    navigateBack: () => ipcRenderer.send('navigate-back'),
    navigateFwd: () => ipcRenderer.send('navigate-fwd'),
    refreshPage: () => ipcRenderer.send('refresh-page'),
    openConfig: () => ipcRenderer.send('open-config'),
    showQueue: () => ipcRenderer.send('show-queue'),
    minimizeTaskbar: () => ipcRenderer.send('minimize-taskbar'),
    restoreTaskbar: () => ipcRenderer.send('restore-taskbar'),
});
