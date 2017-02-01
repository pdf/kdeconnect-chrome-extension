var hostname = 'com.0xc0dedbad.kdeconnect_chrome';
var port = null;
var defaultDeviceId = null;
var defaultOnly = false;
var knownDevices = {};
var reconnectDelay = 100;
var reconnectTimer = null;
var reconnectResetTimer = null;

function toggleAction(tab, forced) {
    if (typeof tab.id !== 'number') {
        console.error('tab.id should be number:', tab)
        return;
    }
    if (typeof tab.url !== 'string') {
        console.error('tab.url should be string:', tab)
        return;
    }

    if (tab.url.indexOf('chrome://') === 0 || forced === true) {
        chrome.browserAction.disable(tab.id);
    } else {
        chrome.browserAction.enable(tab.id);
    }
}

function sendMessage(msg) {
    if (!port || !msg || !msg.type) {
        console.error('Missing message parameters')
    }
    port.postMessage(msg);
}

function onRuntimeMessage(msg, sender, sendResponse) {
    if (sender.url === 'chrome-extension://' + chrome.runtime.id + '/background.html') {
        // Ignore locally generated messages
        return;
    }
    sendMessage(msg);
}

function setWarningBadge(color) {
    chrome.browserAction.getBadgeText({}, function(text) {
        if (text !== '!') {
            chrome.browserAction.setBadgeText({ text: '!' });
            chrome.browserAction.setBadgeBackgroundColor({ color: color });
        }
    });
}

function clearWarningBadge() {
    chrome.browserAction.getBadgeText({}, function(text) {
        if (text !== '') {
            chrome.browserAction.setBadgeText({ text: '' });
            chrome.browserAction.setBadgeBackgroundColor({ color: [0, 0, 0, 0] });
        }
    });
}

function contextMenuHandler(info, tab) {
    sendMessage({
        type: 'typeShare',
        data: {
            target: info.menuItemId,
            url: info.linkUrl || info.srcUrl || info.frameUrl || info.pageUrl
        }
    });
}

function createContextMenus(devices) {
    chrome.contextMenus.removeAll(function() {
        var devs = devices;
        if (defaultOnly && defaultDeviceId) {
            devs = {};
            devs[defaultDeviceId] = devices[defaultDeviceId];
        }
        var keys = Object.keys(devs);
        if (keys.length === 0) {
            return;
        }
        var active = false;
        keys.forEach(function(key) {
            if (devs[key].isReachable && devs[key].isTrusted) {
                active = true;
                return;
            }
        });
        if (!active) {
            setWarningBadge([255, 129, 0, 220]);
            return;
        }
        if (port) {
            clearWarningBadge();
        }

        if (keys.length === 1) {
            var key = keys[0];
            chrome.contextMenus.create({
                id: key,
                title: 'KDE Connect (' + devs[key].name + ')',
                enabled: devs[key].isReachable && devs[key].isTrusted,
                contexts: ['page', 'frame', 'link', 'image', 'video', 'audio'],
                onclick: contextMenuHandler,
            })
            return;
        }

        chrome.contextMenus.create({
            id: 'kdeconnectRoot',
            title: 'KDE Connect',
            contexts: ['page', 'frame', 'link', 'image', 'video', 'audio']
        });
        Object.keys(devs).forEach(function(key) {
            chrome.contextMenus.create({
                id: key,
                title: devs[key].name,
                enabled: devs[key].isReachable && devs[key].isTrusted,
                parentId: 'kdeconnectRoot',
                contexts: ['page', 'frame', 'link', 'image', 'video', 'audio'],
                onclick: contextMenuHandler,
            });
        });
    });
}

function updateContextMenu(device) {
    chrome.contextMenus.update(device.id, {
        title: device.name,
        enabled: device.isReachable && device.isTrusted,
    });
}

function updateDevice(device) {
    var known = knownDevices[device.id]
    knownDevices[device.id] = device;
    if (known) {
        // TODO: Sort out dynamic updates, maybe not until I pull in a framework
        //updateContextMenu(device);
        createContextMenus(knownDevices);
    } else {
        createContextMenus(knownDevices);
    }
}

function changeValue(change) {
    if (change === undefined) {
        return undefined;
    }
    if (change.newValue === undefined) {
        return change.oldValue;
    }
    return change.newValue;
}

function onStorageChanged(changes, areaName) {
    if (areaName != 'sync') {
        return;
    }
    var newDefaultDeviceId = changeValue(changes.defaultDeviceId);
    if (newDefaultDeviceId !== undefined) {
        defaultDeviceId = newDefaultDeviceId;
    }
    var newDefaultOnly = changeValue(changes.defaultOnly);
    if (newDefaultOnly !== undefined) {
        defaultOnly = newDefaultOnly;
    }
    if (defaultOnly && knownDevices[defaultDeviceId]) {
        createContextMenus(knownDevices);
    }
}

function restoreOptions() {
    chrome.storage.onChanged.addListener(onStorageChanged);
    chrome.storage.sync.get({
        defaultOnly: false,
        defaultDeviceId: null,
    }, function(items) {
        onStorageChanged({
            defaultDeviceId: {
                newValue: items.defaultDeviceId
            },
            defaultOnly: {
                newValue: items.defaultOnly
            }
        }, 'sync');
    });
}

function onMessage(msg) {
    switch (msg.type) {
        case 'typeDeviceUpdate':
            updateDevice(msg.data);
            chrome.runtime.sendMessage(msg);
            break;
        case 'typeDevices':
            knownDevices = msg.data;
            createContextMenus(msg.data);
            chrome.runtime.sendMessage(msg);
            break;
        default:
            chrome.runtime.sendMessage(msg);
    }
}

function resetReconnect() {
    reconnectDelay = 100;
}

function onDisconnect() {
    port = null;
    setWarningBadge([255, 0, 0, 220]);
    // Disconnected, cancel back-off reset
    if (typeof reconnectResetTimer === 'number') {
        window.clearTimeout(reconnectResetTimer);
        reconnectResetTimer = null;
    }
    // Don't queue more than one reconnect
    if (typeof reconnectTimer === 'number') {
        window.clearTimeout(reconnectTimer);
        reconnectTimer = null;
    }

    var message;
    if (chrome.runtime.lastError) {
        message = chrome.runtime.lastError.message;
    }
    console.warn('Disconnected from native host: ' + message);

    // Exponential back-off on reconnect
    reconnectTimer = window.setTimeout(function() {
        connect();
    }, reconnectDelay)
    reconnectDelay = reconnectDelay * 2
}

function connect() {
    clearWarningBadge();
    port = chrome.runtime.connectNative(hostname);
    // Reset the back-off delay if we stay connected
    reconnectResetTimer = window.setTimeout(function() {
        reconnectDelay = 100;
    }, reconnectDelay * 0.9);

    port.onDisconnect.addListener(onDisconnect);
    port.onMessage.addListener(onMessage);
    port.postMessage({ type: 'typeDevices' });
}

connect();

restoreOptions();

chrome.runtime.onMessage.addListener(onRuntimeMessage);

chrome.tabs.onActivated.addListener(function(info) {
    chrome.tabs.get(info.tabId, toggleAction);
});

chrome.tabs.onUpdated.addListener(function(tabId, change, tab) {
    toggleAction(tab);
});
