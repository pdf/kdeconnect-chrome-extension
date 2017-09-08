var defaultDeviceId = null;


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

    chrome.storage.sync.set({
        defaultDeviceId: newDefaultDeviceId,
        defaultOnly: newDefaultOnly,
        disableContextMenu: newDisableContextMenu,
    }, function () {
        var status = document.getElementById('status');
        status.textContent = 'Saved...';
        setTimeout(function () {
            status.textContent = '';
        }, 750);
    });
}

function restoreOptions() {
    chrome.storage.sync.get({
        defaultOnly: false,
        defaultDeviceId: null,
        disableContextMenu: false,
    }, function (items) {
        defaultDeviceId = items.defaultDeviceId;
        var defaultOnly = document.getElementById('defaultOnly');
        defaultOnly.checked = items.defaultOnly;
        document.getElementById('defaultDevice').value = items.defaultDeviceId;
        toggleDevices(defaultOnly);
        var disableContextMenu = document.getElementById('disableContextMenu');
        disableContextMenu.checked = items.disableContextMenu;
    });
}


function fetchDevices() {
    chrome.runtime.sendMessage({
        type: 'typeDevices',
    });
}

function onMessage(msg, sender, sendResponse) {
    if (sender.url !== 'chrome-extension://' + chrome.runtime.id + '/background.html') {
        // Messages flow one-way
        return;
    }
    switch (msg.type) {
        case 'typeDevices':
            writeDevices(msg.data);
            break;
        default:
            return;
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

chrome.runtime.onMessage.addListener(onMessage);
