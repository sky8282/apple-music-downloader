// main.js
const { app, BrowserWindow, ipcMain, shell } = require('electron');
const path = require('path');
const fs = require('fs');
const { spawn } = require('child_process');
const MUSIC_URL = 'https://music.apple.com/';

let win; 
let taskbarWin; 
let queueWin; 
let currentServiceUrl = ''; 
let taskQueue = []; 
let activeTask = null; 
let taskIdCounter = 0;

const TASKBAR_HEIGHT_NORMAL = 200;
const TASKBAR_HEIGHT_COLLASPED = 38;
let isTaskbarMinimized = true;

let userConfigPath = '';

function manageConfig() {
    try {
        userConfigPath = path.join(app.getPath('userData'), 'config.yaml');
        if (fs.existsSync(userConfigPath)) {
            console.log(`[Main] 配置文件已存在: ${userConfigPath}`);
            return;
        }

        console.log(`[Main] 配置文件不存在, 正在创建...`);
        
        const isDev = !app.isPackaged;
        let templateConfigPath;

        if (isDev) {
            templateConfigPath = path.join(__dirname, 'config.yaml');
        } else {
            templateConfigPath = path.join(process.resourcesPath, 'config.yaml');
        }

        if (fs.existsSync(templateConfigPath)) {
            fs.copyFileSync(templateConfigPath, userConfigPath);
            console.log(`[Main] 配置文件已从 ${templateConfigPath} 复制到 ${userConfigPath}`);
        } else {
            console.error(`[Main] 无法找到模板配置文件: ${templateConfigPath}`);
        }
    } catch (e) {
        console.error(`[Main] 管理配置文件时出错: ${e}`);
    }
}

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
        height: TASKBAR_HEIGHT_COLLASPED, 
        x: mainBounds.x,
        y: mainBounds.y + mainBounds.height - TASKBAR_HEIGHT_COLLASPED,
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
        const waitingTasks = taskQueue.filter(t => t.status === 'queued').length;
        taskbarWin.webContents.send('set-queue-length', waitingTasks);
    });
}

function broadcast(channel, data) {
    if (taskbarWin) taskbarWin.webContents.send(channel, data);
    if (queueWin) queueWin.webContents.send(channel, data);
}

function getSerializableQueueState() {
    return taskQueue
        .filter(t => t.status !== 'finished')
        .map(t => ({
            id: t.id,
            url: t.url,
            name: t.name,
            status: t.status,
            completedTracks: t.completedTracks,
            totalTracks: t.totalTracks
        }));
}

function broadcastQueueUpdate() {
    const queueState = getSerializableQueueState();
    broadcast('queue-update', queueState);
    
    const waitingTasks = taskQueue.filter(t => t.status === 'queued').length;
    if (taskbarWin) taskbarWin.webContents.send('set-queue-length', waitingTasks);
}

function syncTaskbarPosition() {
    if (!win || !taskbarWin) return; 
    
    const currentHeight = isTaskbarMinimized ? TASKBAR_HEIGHT_COLLASPED : TASKBAR_HEIGHT_NORMAL;
    
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
    manageConfig();
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

function addTask(url, name) {
    if (activeTask && activeTask.url === url) {
        console.log(`[Main] 忽略重复任务 (已在运行): ${url}`);
        return; 
    }
    
    const isQueued = taskQueue.some(task => task.url === url && task.status !== 'finished');
    if (isQueued) {
        console.log(`[Main] 忽略重复任务 (已在队列): ${url}`);
        return;
    }
    
    taskIdCounter++;
    const newTask = {
        id: `task_${taskIdCounter}`,
        url: url,
        name: name,
        status: 'queued',
        completedTracks: 0,
        totalTracks: 0,
        taskDataStore: {},
        process: null
    };
    
    console.log(`[Main] 添加新任务到队列: ${newTask.name}`); 
    taskQueue.push(newTask);
    broadcast('task-queued', newTask);
    broadcastQueueUpdate();
    
    if (!activeTask) {
        setTimeout(processQueue, 0);
    }
}

ipcMain.on('start-download', (event, { url, details }) => {
    if (win && event.sender === win.webContents) { 
        addTask(url, details.name || url.split('/').pop());
    }
});

ipcMain.on('start-download-url', (event, url) => {
    if (!url) return;
    
    let name = url;
    try {
        const urlObj = new URL(url);
        const parts = urlObj.pathname.split('/').filter(Boolean);
        let potentialName = parts[parts.length - 2]; 
        let potentialId = parts[parts.length - 1];   

        if (potentialName && !isNaN(Number(potentialId)) && isNaN(Number(potentialName))) {
            name = decodeURIComponent(potentialName);
        } else {
            let lastName = parts.pop();
            if (lastName) {
                 name = decodeURIComponent(lastName.split('?i=')[0]);
            } else {
                 name = url;
            }
        }
    } catch(e) {
        try {
            name = decodeURIComponent(url.split('/').filter(Boolean).pop());
        } catch(e2) {}
    }

    addTask(url, name);
});

ipcMain.on('request-next-task', () => {
    console.log('[Main] UI 已就绪, 检查下一个任务...');
    processQueue();
});

ipcMain.on('cancel-task', (event, taskId) => {
    console.log(`[Main] 收到取消请求: ${taskId}`);
    
    const taskToCancel = taskQueue.find(t => t.id === taskId);
    if (!taskToCancel) return;

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
        activeTask.status = 'finished';
        activeTask = null;
    } else {
        taskToCancel.status = 'finished';
    }
    
    broadcast('task-cancelled', taskId);
    broadcastQueueUpdate();
    
    setTimeout(processQueue, 0);
});

ipcMain.on('navigate-back', () => win && win.webContents.goBack()); 
ipcMain.on('navigate-fwd', () => win && win.webContents.goForward()); 
ipcMain.on('refresh-page', () => win && win.webContents.reloadIgnoringCache());
ipcMain.on('open-config', () => {
    if (userConfigPath) {
        shell.openPath(userConfigPath);
    } else {
        console.error('[Main] userConfigPath 为空, 无法打开。');
    }
});

ipcMain.on('show-queue', (event) => {
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
        const queueState = getSerializableQueueState();
        queueWin.webContents.send('queue-snapshot', queueState);
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
    if (activeTask) {
        return; 
    }
    
    const taskToRun = taskQueue.find(t => t.status === 'queued');
    if (!taskToRun) {
        return;
    }
        
    activeTask = taskToRun;
    activeTask.status = 'running';
    
    console.log(`[Main] 准备启动任务: ${activeTask.id} (Name: ${activeTask.name})`);
    broadcast('task-started', activeTask);
    broadcastQueueUpdate();

    if (isTaskbarMinimized) {
        isTaskbarMinimized = false;
        syncTaskbarPosition();
        broadcast('set-taskbar-state', false); 
    }
    
    const isDev = !app.isPackaged;
    let goProcess;
    
    if (isDev) {
        console.log('[Main] DEV Mode: Spawning with `go run main.go`');
        const goArgs = ['run', 'main.go', '--json-output', '--config', userConfigPath];
        if (activeTask.url.includes('?i=') || activeTask.url.includes('/song/')) {
            goArgs.push('--song');
        }
        goArgs.push(activeTask.url);
        
        console.log(`[Main] Spawning Go with args: ${goArgs.join(' ')}`);
        goProcess = spawn('go', goArgs, { 
            cwd: __dirname,
            shell: false,
            detached: true
        });

    } else {
        console.log('[Main] PROD Mode: Spawning pre-compiled binary');
        const binaryName = process.platform === 'win32' ? 'downloader.exe' : 'downloader';
        const goBinDir = process.resourcesPath;
        let appCwd;
        if (process.platform === 'win32') {
            appCwd = path.dirname(app.getPath('exe'));
        } else {
            appCwd = path.join(app.getPath('exe'), '..', '..', '..', '..');
        }

        const binaryPath = path.join(goBinDir, binaryName);
        const goArgs = ['--json-output', '--config', userConfigPath];

        if (activeTask.url.includes('?i=') || activeTask.url.includes('/song/')) {
            goArgs.push('--song');
        }
        goArgs.push(activeTask.url);
        
        console.log(`[Main] Binary Path: ${binaryPath}`); 
        console.log(`[Main] Working Dir (CWD): ${appCwd}`);
        const spawnEnv = {
            ...process.env, 
            'PATH': [
                goBinDir, 
                process.env.PATH
            ].join(process.platform === 'win32' ? ';' : ':')
        };
        
        goProcess = spawn(binaryPath, goArgs, { 
            cwd: appCwd, 
            shell: false,
            detached: true,
            env: spawnEnv
        });
    }
    
    activeTask.process = goProcess;
    
    goProcess.stdout.on('data', (data) => {
        const output = data.toString();
        output.split('\n').forEach(line => {
            if (line.trim()) {
                let json;
                try {
                    json = JSON.parse(line);
                } catch (e) {
                    broadcast('go-output', `{\"taskId\": \"${taskToRun.id}\", \"status\": \"log\", \"message\": \"${line.replace(/"/g, '\\"')}\"}`);
                    return;
                }
                
                json.taskId = taskToRun.id; 
                
                if (json.albumName) {
                    taskToRun.name = json.albumName;
                } else if (json.albumId) {
                    json.albumName = taskToRun.name; 
                }

                broadcast('go-output', JSON.stringify(json));
                
                if (json.trackNum !== undefined && json.trackNum > 0) {
                    const trackNum = parseInt(json.trackNum, 10);
                    if (trackNum > taskToRun.totalTracks) {
                        taskToRun.totalTracks = trackNum;
                    }

                    const track = taskToRun.taskDataStore[trackNum];
                    if (!track) {
                        taskToRun.taskDataStore[trackNum] = { status: 'start' };
                    }
                    
                    if ((json.status === 'complete' || json.status === 'exists' || json.status === 'error') &&
                        (!track || track.status !== 'finished')) {
                        taskToRun.completedTracks++;
                        taskToRun.taskDataStore[trackNum] = { status: 'finished' }; 
                    }
                    
                    broadcast('task-progress-update', { 
                        id: taskToRun.id, 
                        completed: taskToRun.completedTracks, 
                        total: taskToRun.totalTracks 
                    });
                    broadcastQueueUpdate();
                }
            }
        });
    });

    goProcess.stderr.on('data', (data) => {
        broadcast('go-output', `{\"taskId\": \"${taskToRun.id}\", \"status\": "error", \"message\": \"${data.toString().replace(/"/g, '\\"')}\"}`);
    });

    goProcess.on('close', (code) => {
        console.log(`[Go ${taskToRun.id}] 进程退出，代码 ${code}`); 
        
        if (activeTask && activeTask.id === taskToRun.id) {
            taskToRun.status = 'finished';
            broadcast('task-finished', taskToRun.id);
            activeTask = null; 
            broadcastQueueUpdate();
            
            if (taskQueue.filter(t => t.status !== 'finished').length === 0 && !isTaskbarMinimized) {
                isTaskbarMinimized = true;
                syncTaskbarPosition();
                broadcast('set-taskbar-state', true); 
            }
            processQueue();
        } else if (!activeTask) {
            taskToRun.status = 'finished';
            broadcastQueueUpdate();
        }
    });
    
    goProcess.on('error', (err) => {
        const errMsg = err.message ? err.message.replace(/"/g, '\\"') : "未知进程错误";
        console.error(`[Main] 启动 Go 进程失败:`, err);
        
        broadcast('go-output', `{\"taskId\": \"${taskToRun.id}\", \"status\": "error", \"message\": "Go进程启动失败: ${errMsg}"}`);
        
        if (activeTask && activeTask.id === taskToRun.id) {
            taskToRun.status = 'finished'; 
            broadcast('task-finished', taskToRun.id);
            activeTask = null; 
            broadcastQueueUpdate();
            
            if (taskQueue.filter(t => t.status !== 'finished').length === 0 && !isTaskbarMinimized) {
                isTaskbarMinimized = true;
                syncTaskbarPosition();
                broadcast('set-taskbar-state', true); 
            }
            processQueue();
        } else if (!activeTask) {
            taskToRun.status = 'finished';
            broadcastQueueUpdate();
        }
    });
}
