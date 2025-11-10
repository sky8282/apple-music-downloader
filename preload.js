// preload.js
const { contextBridge, ipcRenderer } = require('electron');
contextBridge.exposeInMainWorld('desktopApp', {
    download: (url, details) => {
        ipcRenderer.send('start-download', { url, details });
    },
    onGoOutput: (callback) => ipcRenderer.on('go-output', (e, msg) => callback(msg)),
    onTaskQueued: (callback) => ipcRenderer.on('task-queued', (e, task) => callback(task)),
    onTaskCancelled: (callback) => ipcRenderer.on('task-cancelled', (e, id) => callback(id)),
    onTaskStarted: (callback) => ipcRenderer.on('task-started', (e, task) => callback(task)),
    onTaskFinished: (callback) => ipcRenderer.on('task-finished', (e, id) => callback(id)),
    onQueueSnapshot: (callback) => ipcRenderer.on('queue-snapshot', (e, queue) => callback(queue)),
    onSetTaskbarState: (callback) => ipcRenderer.on('set-taskbar-state', (e, isMinimized) => callback(isMinimized)),
    onSetActiveService: (callback) => ipcRenderer.on('set-active-service', (e, url) => callback(url)),
    switchService: (url) => ipcRenderer.send('switch-service', url),
    cancelTask: (taskId) => ipcRenderer.send('cancel-task', taskId),
    navigateBack: () => ipcRenderer.send('navigate-back'),
    navigateFwd: () => ipcRenderer.send('navigate-fwd'),
    showQueue: () => ipcRenderer.send('show-queue'),
    minimizeTaskbar: () => ipcRenderer.send('minimize-taskbar'),
    restoreTaskbar: () => ipcRenderer.send('restore-taskbar'),
});