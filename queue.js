// queue.js
const queueList = document.getElementById('queue-modal-list');
let queue = [];

window.desktopApp.onQueueSnapshot((initialQueue) => {
    queue = initialQueue;
    render();
});
window.desktopApp.onTaskQueued((task) => {
    queue.push(task);
    render();
});
window.desktopApp.onTaskCancelled((taskId) => {
    queue = queue.filter(t => t.id !== taskId);
    render();
});
window.desktopApp.onTaskStarted((task) => {
    queue = queue.filter(t => t.id !== task.id);
    render();
});

function render() {
    queueList.innerHTML = '';
    queue.forEach(task => {
        const item = document.createElement('div');
        item.className = 'queue-item';
        item.id = `queue-item-${task.id}`;
        item.innerHTML = `
            <span class="queue-item-name" title="${task.name}">${task.name}</span>
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