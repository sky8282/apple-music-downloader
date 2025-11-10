// main.js
const { app, BrowserWindow, ipcMain } = require('electron');
const path = require('path');
const fs = require('fs');
const { spawn } = require('child_process');
const GO_PROJECT_DIR = __dirname;
const MUSIC_URL = 'https://music.apple.com/';
let win;
let taskbarWin;
let queueWin;
let currentServiceUrl = ''; 
let taskQueue = [];
let activeTask = null;
let taskIdCounter = 0;

const TASKBAR_HEIGHT_NORMAL = 200;
const TASKBAR_HEIGHT_COLLAPSED = 38;
let isTaskbarMinimized = true;

function createMainWindow(urlToLoad) {
    win = new BrowserWindow({
        width: 1600,
        height: 1000,
        webPreferences: {
            preload: path.join(__dirname, 'preload.js'),
            contextIsolation: true,
            nodeIntegration: false
        }
    });

    win.loadURL(urlToLoad); 
    currentServiceUrl = urlToLoad; 
    win.webContents.on('did-finish-load', () => {
        fs.readFile(path.join(__dirname, 'injector.js'), 'utf-8', (err, script) => {
            if (err) { console.error('无法加载 injector.js', err); return; }
            win.webContents.executeJavaScript(script);
        });
    });

    createTaskbarWindow();
    win.on('resize', syncTaskbarPosition);
    win.on('move', syncTaskbarPosition);
    win.on('close', () => app.quit());
}

function createTaskbarWindow() {
    const mainBounds = win.getBounds();
    taskbarWin = new BrowserWindow({
        width: mainBounds.width,
        height: TASKBAR_HEIGHT_COLLAPSED, 
        x: mainBounds.x,
        y: mainBounds.y + mainBounds.height - TASKBAR_HEIGHT_COLLAPSED,
        parent: win,
        frame: false,
        transparent: true,
        alwaysOnTop: true,
        skipTaskbar: true,
        resizable: false,
        maximizable: false,
        webPreferences: {
            preload: path.join(__dirname, 'preload.js'),
            contextIsolation: true,
            nodeIntegration: false
        }
    });

    taskbarWin.loadFile(path.join(__dirname, 'index.html'), { hash: 'taskbar' });
    
    taskbarWin.webContents.on('did-finish-load', () => {
        taskbarWin.webContents.send('set-taskbar-state', isTaskbarMinimized);
        taskbarWin.webContents.send('set-active-service', currentServiceUrl); 
    });
}

function broadcast(channel, data) {
    if (taskbarWin) taskbarWin.webContents.send(channel, data);
    if (queueWin) queueWin.webContents.send(channel, data);
}

function syncTaskbarPosition() {
    if (!win || !taskbarWin) return; 
    const currentHeight = isTaskbarMinimized ? TASKBAR_HEIGHT_COLLAPSED : TASKBAR_HEIGHT_NORMAL;
    const [width, ] = win.getSize();
    const [x, y] = win.getPosition();
    const mainBounds = win.getBounds();
    taskbarWin.setBounds({
        x: x,
        y: y + mainBounds.height - currentHeight,
        width: width,
        height: currentHeight
    });
}

app.whenReady().then(() => {
    createMainWindow(MUSIC_URL);
});
ipcMain.on('switch-service', (event, urlToLoad) => {
    if (win && currentServiceUrl !== urlToLoad) {
        console.log(`[Main] 切换服务至: ${urlToLoad}`);
        win.loadURL(urlToLoad);
        currentServiceUrl = urlToLoad;
        broadcast('set-active-service', currentServiceUrl);
    }
});

ipcMain.on('start-download', (event, { url, details }) => {
    if (win && event.sender === win.webContents) { 
        taskIdCounter++;
        const newTask = {
            id: `task_${taskIdCounter}`,
            url: url,
            name: details.name || url.split('/').pop()
        };
        taskQueue.push(newTask);
        broadcast('task-queued', newTask);
        processQueue();
    }
});

ipcMain.on('cancel-task', (event, taskId) => {
    console.log(`[Main] 收到取消请求: ${taskId}`);
    if (activeTask && activeTask.id === taskId) {
        if (activeTask.process && activeTask.process.pid) {
            console.log(`[Main] 正在终止活动任务组 (PID: ${activeTask.process.pid})`);
            try {
                if (process.platform === 'win32') {
                    spawn('taskkill', ['/PID', activeTask.process.pid, '/F', '/T']);
                } else {
                    process.kill(-activeTask.process.pid, 'SIGKILL');
                }
            } catch (e) {
                console.error(`[Main] 终止进程组 ${activeTask.process.pid} 失败, 尝试后备方案:`, e.message);
                activeTask.process.kill('SIGKILL');
            }
        }
    } else {
        taskQueue = taskQueue.filter(task => task.id !== taskId);
        console.log(`[Main] 从队列移除: ${taskId}`);
        broadcast('task-cancelled', taskId);
    }
});

ipcMain.on('navigate-back', () => win && win.webContents.goBack()); 
ipcMain.on('navigate-fwd', () => win && win.webContents.goForward()); 
ipcMain.on('show-queue', () => {
    if (queueWin) {
        queueWin.focus();
        return;
    }
    queueWin = new BrowserWindow({
        width: 500,
        height: 400,
        title: "下载队列",
        webPreferences: {
            preload: path.join(__dirname, 'preload.js'),
            contextIsolation: true,
            nodeIntegration: false
        }
    });
    
    queueWin.loadFile(path.join(__dirname, 'index.html'), { hash: 'queue' });
    queueWin.webContents.on('did-finish-load', () => {
        queueWin.webContents.send('queue-snapshot', taskQueue);
    });
    queueWin.on('closed', () => {
        queueWin = null;
    });
});

ipcMain.on('minimize-taskbar', () => {
    isTaskbarMinimized = true;
    syncTaskbarPosition();
});

ipcMain.on('restore-taskbar', () => {
    isTaskbarMinimized = false;
    syncTaskbarPosition();
});

function processQueue() {
    if (!activeTask && taskQueue.length > 0) {
        const taskToRun = taskQueue.shift();
        activeTask = { ...taskToRun, process: null };
        console.log(`[Main] 准备启动任务: ${activeTask.id} (Name: ${activeTask.name})`);
        broadcast('task-started', activeTask);
        if (isTaskbarMinimized) {
            isTaskbarMinimized = false;
            syncTaskbarPosition();
            broadcast('set-taskbar-state', false); 
        }
        
        const goArgs = ['run', 'main.go', '--json-output'];
        if (activeTask.url.includes('?i=') || activeTask.url.includes('/song/')) {
            goArgs.push('--song');
        }
        goArgs.push(activeTask.url);
        console.log(`[Main] Spawning Go with args: ${goArgs.join(' ')}`); 
        const goProcess = spawn('go', goArgs, { 
            cwd: GO_PROJECT_DIR,
            shell: false,
            detached: true
        });
        
        activeTask.process = goProcess;
        goProcess.stdout.on('data', (data) => {
            const output = data.toString();
            output.split('\n').forEach(line => {
                if (line.trim()) {
                    try {
                        const json = JSON.parse(line);
                        json.taskId = activeTask.id;
                        json.albumName = activeTask.name;
                        broadcast('go-output', JSON.stringify(json));
                    } catch (e) {
                        broadcast('go-output', `{\"taskId\": \"${activeTask.id}\", \"status\": \"log\", \"message\": \"${line.replace(/"/g, '\\"')}\"}`);
                    }
                }
            });
        });

        goProcess.stderr.on('data', (data) => {
            broadcast('go-output', `{\"taskId\": \"${activeTask.id}\", \"status\": "error", \"message\": \"${data.toString().replace(/"/g, '\\"')}\"}`);
        });

        goProcess.on('close', (code) => {
            console.log(`[Go ${activeTask.id}] 进程退出，代码 ${code}`);
            if (activeTask && activeTask.id === taskToRun.id) {
                broadcast('task-finished', activeTask.id);
                activeTask = null;
                if (taskQueue.length === 0 && !isTaskbarMinimized) {
                    isTaskbarMinimized = true;
                    syncTaskbarPosition();
                    broadcast('set-taskbar-state', true); 
                }
                processQueue();
            }
        });
        
        goProcess.on('error', (err) => {
            broadcast('go-output', `{\"taskId\": \"${activeTask.id}\", \"status\": "error", \"message\": "Go进程启动失败: ${err.message}"}`);
            if (activeTask && activeTask.id === taskToRun.id) {
                activeTask = null;
                if (taskQueue.length === 0 && !isTaskbarMinimized) {
                    isTaskbarMinimized = true;
                    syncTaskbarPosition();
                    broadcast('set-taskbar-state', true); 
                }
                processQueue();
            }
        });
    }
}