var defaultDeviceId = null;


function writeDevices(devices) {
    var devNode = document.getElementById('defaultDevice');
    devNode.innerHTML = '';
    Object.keys(devices).forEach(function(key) {
        var dev = devices[key];
        var disabled = (!(dev.isReachable || dev.isTrusted)) ? 'disabled' : null;
        var selected = defaultDeviceId == dev.id ? 'selected' : null;
        devNode.innerHTML += '<option value="' + dev.id + '" ' + disabled + ' ' + selected + '>' + dev.name + '</option>';
    });
}

function toggleDevices(defaultOnly) {
    document.getElementById('defaultDeviceContainer').style.display = defaultOnly.checked ? 'inherit' : 'none';
}

function saveOptions() {
    var newDefaultDeviceId = document.getElementById('defaultDevice').value;
    var newDefaultOnly = document.getElementById('defaultOnly').checked;

    chrome.storage.sync.set({
        defaultDeviceId: newDefaultDeviceId,
        defaultOnly: newDefaultOnly,
    }, function() {
        var status = document.getElementById('status');
        status.textContent = 'Saved...';
        setTimeout(function() {
            status.innerHTML = '&nbsp;';
        }, 750);
    });
}

function restoreOptions() {
    chrome.storage.sync.get({
        defaultOnly: false,
        defaultDeviceId: null,
    }, function(items) {
        defaultDeviceId = items.defaultDeviceId;
        var defaultOnly = document.getElementById('defaultOnly');
        defaultOnly.checked = items.defaultOnly;
        document.getElementById('defaultDevice').value = items.defaultDeviceId;
        toggleDevices(defaultOnly);
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

document.addEventListener('DOMContentLoaded', function() {
    restoreOptions();
    fetchDevices();
    document.getElementById('save').addEventListener('click', saveOptions);
    document.getElementById('defaultOnly').addEventListener('change', function(event) {
        toggleDevices(event.target);
    });
});

chrome.runtime.onMessage.addListener(onMessage);
