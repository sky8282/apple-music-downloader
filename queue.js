// queue.js
const queueList = document.getElementById('queue-modal-list');
let queue = [];

window.desktopApp.onQueueSnapshot((initialQueue) => {
    queue = initialQueue;
    render();
});

window.desktopApp.onQueueUpdate((updatedQueue) => {
    queue = updatedQueue;
    render();
});

function render() {
    queueList.innerHTML = '';
    
    const tasksToRender = queue.filter(t => t.status !== 'finished');

    tasksToRender.forEach((task, index) => {
        const item = document.createElement('div');
        item.className = 'queue-item';
        item.id = `queue-item-${task.id}`;
        
        let progressText = '(等待中)';
        if (task.status === 'running') {
            progressText = `(${task.completedTracks || 0} / ${task.totalTracks || '?'})`;
        }

        const displayName = `${index + 1}. ${task.name} ${progressText}`;
        
        item.innerHTML = `
            <span class="queue-item-name" title="${displayName}">${displayName}</span>
            <button class="task-cancel-button" data-task-id="${task.id}">&times;</button>
        `;
        queueList.appendChild(item);
    });
    
    queueList.querySelectorAll('.task-cancel-button').forEach(button => {
        button.addEventListener('click', (e) => {
            const taskId = e.target.dataset.taskId;
            window.desktopApp.cancelTask(taskId);
        });
    });
}
