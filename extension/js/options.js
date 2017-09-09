var defaultDeviceId = null;

function logError(error) {
    // Suppress errors caused by Mozilla polyfill
    // TODO: Fix these somehow?
    if (
        error.message !== 'Could not establish connection. Receiving end does not exist.' &&
        error.message !== 'The message port closed before a response was received.'
    ) {
        console.error(error.message)
    }
}

function sendMessage(msg) {
    browser.runtime.sendMessage(msg).then(function () { return true; }).catch(logError)
}

function writeDevices(devices) {
    var devNode = document.getElementById('defaultDevice');
    while (devNode.hasChildNodes()) {
        devNode.removeChild(devNode.lastChild);
    }
    Object.keys(devices).forEach(function (key) {
        var dev = devices[key];
        var opt = document.createElement('option');
        opt.value = dev.id;
        opt.disabled = (!(dev.isReachable || dev.isTrusted));
        opt.selected = defaultDeviceId == dev.id;
        opt.textContent = dev.name;
        devNode.appendChild(opt);
    });
}

function toggleDevices(defaultOnly) {
    document.getElementById('defaultDeviceContainer').style.display = defaultOnly.checked ? 'inherit' : 'none';
}

function saveOptions() {
    var newDefaultDeviceId = document.getElementById('defaultDevice').value;
    var newDefaultOnly = document.getElementById('defaultOnly').checked;
    var newDisableContextMenu = document.getElementById('disableContextMenu').checked;

    browser.storage.sync.set({
        defaultDeviceId: newDefaultDeviceId,
        defaultOnly: newDefaultOnly,
        disableContextMenu: newDisableContextMenu,
    }).then(function () {
        var status = document.getElementById('status');
        status.textContent = 'Saved...';
        setTimeout(function () {
            status.textContent = '';
        }, 750);
    }).catch(function (error) {
        var status = document.getElementById('status');
        status.textContent = 'Error: ' + error.message;
    });
}

function restoreOptions() {
    browser.storage.sync.get({
        defaultOnly: false,
        defaultDeviceId: null,
        disableContextMenu: false,
    }).then(function (items) {
        defaultDeviceId = items.defaultDeviceId;
        var defaultOnly = document.getElementById('defaultOnly');
        defaultOnly.checked = items.defaultOnly;
        document.getElementById('defaultDevice').value = items.defaultDeviceId;
        toggleDevices(defaultOnly);
        var disableContextMenu = document.getElementById('disableContextMenu');
        disableContextMenu.checked = items.disableContextMenu;
    }).catch(function (error) {
        console.error('Error restoring options: ' + error.message);
    });
}


function fetchDevices() {
    sendMessage({
        type: 'typeDevices',
    });
}

function onMessage(msg, sender) {
    if (sender.url.indexOf('/background.html') < 0) {
        // Messages flow one-way
        return;
    }
    switch (msg.type) {
        case 'typeDevices':
            writeDevices(msg.data);
            break;
        default:
            return Promise.resolve();
    }
}

document.addEventListener('DOMContentLoaded', function () {
    restoreOptions();
    fetchDevices();
    document.getElementById('save').addEventListener('click', saveOptions);
    document.getElementById('defaultOnly').addEventListener('change', function (event) {
        toggleDevices(event.target);
    });
});

browser.runtime.onMessage.addListener(onMessage);
