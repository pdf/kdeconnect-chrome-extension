var currentUrl = null;
var knownDevices = {};
var lastHostVersion = '0.0.5';

function getCurrentTab(callback) {
    chrome.tabs.query({ active: true, currentWindow: true }, function(tabs) {
        if (tabs.length === 0) {
            return;
        }
        callback(tabs[0]);
    });
}

function sendUrlCallback(target, url) {
    return function() {
        sendUrl(target, url);
    }
}

function sendUrl(target, url) {
    if (!target || !url) {
        console.warn('Missing params for sendUrl');
    }
    chrome.runtime.sendMessage({
        type: 'typeShare',
        data: {
            target: target,
            url: url,
        },
    });
    window.close();
}

function writeDevices(devices) {
    var devNode = document.getElementById('devices');
    var keys = Object.keys(devices);
    if (keys.length === 0) {
        devNode.innerHTML = '<small><i>No devices found...</i></small>';
        return;
    }
    devNode.innerHTML = '';
    keys.forEach(function(key) {
        devNode.innerHTML += renderDevice(devices[key]);
        attachDeviceListener(key);
    });
}

function writeStatus(details) {
    var devNode = document.getElementById('status');
    if (!details) {
        devNode.innerHTML = '';
        return;
    }
    if (details.update) {
        devNode.innerHTML = '<p class="status">A host upgrade v' + details.update + ' is available, please follow the <a target="_blank" href="https://github.com/pdf/kdeconnect-chrome-extension#upgrading">upgrade instructions</a>.</p>'
    }
}

function renderDevice(device) {
    var disabled = (!(device.isReachable && device.isTrusted)) ? 'disabled' : null;
    var icon = device.statusIconName || 'smartphone-connected';
    if (disabled) {
        icon = device.iconName || 'smartphone-disconnected';
    }
    return '<div id="' + device.id + '" class="device"><img class="status-icon" src="images/' + icon + '.svg" /><span>' + device.name + '</span><button ' + disabled + ' data-target="' + device.id + '">Send</button></div>';
}

function attachDeviceListener(id) {
    document.querySelector('button[data-target="' + id + '"]').addEventListener('click', sendUrlCallback(id, currentUrl));
}

function updateDeviceMarkup(device) {
    document.getElementById(device.id).replaceWith(renderDevice(device));
    attachDeviceListener(device.id);
}

function updateDevice(device) {
    var known = knownDevices[device.id];
    knownDevices[device.id] = device;
    if (known) {
        // TODO: Sort out dynamic updates, maybe not until I pull in a framework
        // updateDeviceMarkup(device);
        writeDevices(knownDevices);
    } else {
        fetchDevices();
    }
}

function fetchDevices() {
    chrome.runtime.sendMessage({
        type: 'typeDevices',
    });
}

function fetchVersion() {
    chrome.runtime.sendMessage({
        type: 'typeVersion',
    });
}

function onMessage(msg, sender, sendResponse) {
    if (sender.url !== 'chrome-extension://' + chrome.runtime.id + '/background.html') {
        // Messages flow one-way
        return;
    }
    switch (msg.type) {
        case 'typeDeviceUpdate':
            updateDevice(msg.data);
            break;
        case 'typeDevices':
            knownDevices = msg.data;
            writeDevices(msg.data);
            break;
        case 'typeVersion':
            var version = chrome.runtime.getManifest().version;
            if (lastHostVersion) {
                version = lastHostVersion;
            }
            if (msg.data != version) {
                writeStatus({ update: version });
            } else {
                writeStatus();
            }
        default:
            return;
    }
}

document.addEventListener('DOMContentLoaded', function() {
    chrome.storage.sync.get({
        defaultOnly: false,
        defaultDeviceId: null,
    }, function(items) {
        if (items.defaultOnly && items.defaultDeviceId) {
            getCurrentTab(function(tab) {
                if (!tab) {
                    console.warn('Missing tab?!');
                    return;
                }
                currentUrl = tab.url;
                sendUrl(items.defaultDeviceId, currentUrl);
            });
            return;
        }
        fetchVersion();
        fetchDevices();
        getCurrentTab(function(tab) {
            if (!tab) {
                console.warn('Missing tab?!');
                return;
            }
            currentUrl = tab.url;
        });
    });
});

chrome.runtime.onMessage.addListener(onMessage);
