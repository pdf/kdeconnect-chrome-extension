var currentUrl = null;
var knownDevices = {};

function logError(error) {
    // Suppress errors caused by Mozilla polyfill
    // TODO: Fix these somehow?
    if (
        error.message !== 'Could not establish connection. Receiving end does not exist.' &&
        error.message !== 'The message port closed before a response was received.'
    ) {
        console.error(error.message);
    }
}

function sendMessage(msg) {
    browser.runtime.sendMessage(msg).then(function () { return true; }).catch(logError);
}

function getCurrentTab(callback) {
    browser.tabs.query({ active: true, currentWindow: true }).then(function (tabs) {
        if (tabs.length === 0) {
            return;
        }
        callback(tabs[0]);
    });
}

function sendUrlCallback(target, url) {
    return function () {
        sendUrl(target, url);
    }
}

function sendUrl(target, url) {
    if (!target || !url) {
        console.warn('Missing params for sendUrl');
    }
    sendMessage({
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
    while (devNode.hasChildNodes()) {
        devNode.removeChild(devNode.lastChild);
    }
    var keys = Object.keys(devices);
    if (keys.length === 0) {
        var notFound = document.createElement('small');
        var i = document.createElement('i');
        i.textContent = 'No devices found...';
        notFound.appendChild(i);
        devNode.appendChild(notfound);
        return;
    }
    keys.forEach(function (key) {
        var dev = renderDevice(devices[key]);
        if (dev !== null) {
            devNode.appendChild(dev);
            attachDeviceListener(key);
        }
    });
}

function addStatus(msg) {
    if (!msg || !msg.data || !msg.data.key) {
        return;
    }
    var devNode = document.getElementById('status');
    var p = document.getElementById('status-' + msg.data.key);
    if (!p) {
        p = document.createElement('p');
    }
    p.innerHTML = '';
    p.id = 'status-' + msg.data.key;
    p.className = 'status';
    var leader = document.createElement('span');
    p.appendChild(leader);
    switch (msg.data.type) {
        case 'typeVersion':
            leader.textContent = 'A host upgrade v' + msg.data.update + ' is available (current: v' + msg.data.current + '), please follow the ';
            var link = document.createElement('a');
            link.target = '_blank';
            link.href = 'https://github.com/pdf/kdeconnect-chrome-extension#upgrading';
            link.textContent = 'upgrade instructions';
            p.appendChild(link);
            break;
        case 'typeError':
            leader.style = 'color: red;'
            leader.textContent = 'Error: ' + msg.data.error;
            break;
    }
    devNode.appendChild(p);
}

function removeStatus(msg) {
    if (!msg || !msg.data || !msg.data.key) {
        return;
    }
    var devNode = document.getElementById('status');
    var p = document.getElementById('status-' + msg.data.key);
    if (p) {
        devNode.removeChild(p);
    }
}

function renderDevice(device) {
    if (device === null || device === undefined) {
        return null;
    }
    var devNode = document.createElement('div');
    devNode.setAttribute('id', device.id);
    devNode.disabled = (!(device.isReachable && device.isTrusted));
    devNode.className = 'device';
    var iconName = device.statusIconName || 'smartphone-connected';
    if (devNode.disabled) {
        iconName = device.iconName || 'smartphone-disconnected';
    }
    var icon = document.createElement('img');
    icon.className = 'status-icon';
    icon.src = 'images/' + iconName + '.svg';
    devNode.appendChild(icon);
    var txt = document.createElement('span');
    txt.textContent = device.name;
    devNode.appendChild(txt);
    var btn = document.createElement('button');
    btn.disabled = devNode.disabled;
    btn.dataset.target = device.id;
    btn.textContent = 'Send';
    devNode.appendChild(btn);
    return devNode;
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
    sendMessage({
        type: 'typeDevices',
    });
}

function fetchVersion() {
    sendMessage({
        type: 'typeVersion',
    });
}

function onMessage(msg, sender) {
    if (sender.url.indexOf('/background.html') < 0) {
        // Messages flow one-way
        return Promise.resolve();
    }
    switch (msg.type) {
        case 'typeDeviceUpdate':
            updateDevice(msg.data);
            break;
        case 'typeDevices':
            knownDevices = msg.data;
            writeDevices(msg.data);
            break;
        case 'typeStatus':
            addStatus(msg);
            break;
        case 'typeClearStatus':
            removeStatus(msg);
            break;
        default:
            return Promise.resolve();
    }
}

document.addEventListener('DOMContentLoaded', function () {
    browser.storage.sync.get({
        defaultOnly: false,
        defaultDeviceId: null,
    }).then(function (items) {
        if (items.defaultOnly && items.defaultDeviceId) {
            getCurrentTab(function (tab) {
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
        getCurrentTab(function (tab) {
            if (!tab) {
                console.warn('Missing tab?!');
                return;
            }
            currentUrl = tab.url;
        });
    });
});

browser.runtime.onMessage.addListener(onMessage);
