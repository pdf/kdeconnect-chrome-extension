var hostname = 'com.0xc0dedbad.kdeconnect_chrome';
var port = null;
var defaultDeviceId = null;
var defaultOnly = false;
var disableContextMenu = false;
var knownDevices = {};
var reconnectDelay = 100;
var reconnectTimer = null;
var reconnectResetTimer = null;
var updatePending = null;
var versionTimer = null;

var badges = {};

var red = [255, 0, 0, 220];
var orange = [255, 129, 0, 220];
var blue = [0, 116, 255, 220];
var protocolVersion = '0.1.3';

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

function toggleAction(tab, forced) {
    if (!tab) {
        console.error(`Missing tab for toggleAction`);
        return;
    }
    if (typeof tab.id !== 'number') {
        console.error('tab.id should be number:', tab);
        return;
    }
    if (typeof tab.url !== 'string') {
        console.error('tab.url should be string:', tab);
        return;
    }

    if (tab.url.indexOf('chrome://') === 0 || tab.url.indexOf('about:') === 0 || forced === true) {
        browser.browserAction.disable(tab.id);
    } else {
        browser.browserAction.enable(tab.id);
    }
}

function postMessage(msg) {
    if (!port || !msg || !msg.type) {
        console.error('Missing message parameters', msg);
    }
    port.postMessage(msg);
}

function sendMessage(msg) {
    browser.runtime.sendMessage(msg).then(function () { return true; }).catch(logError);
}

function onMessage(msg, sender, sendResponse) {
    if (sender.url.indexOf('/background.html') > 0) {
        // Ignore locally generated messages
        return Promise.resolve();
    }
    switch (msg.type) {
        case 'typeVersion':
            postMessage(msg);
            versionTimeout();
            break;
        default:
            postMessage(msg);
    }
    return Promise.resolve();
}

function updateBadge(text, color) {
    if (text === undefined || color === undefined) {
        console.error('Missing params for updateBadge');
        return;
    }
    browser.browserAction.getBadgeBackgroundColor({}).then(function (oldColor) {
        if (oldColor !== color) {
            browser.browserAction.setBadgeText({ text: text });
            browser.browserAction.setBadgeBackgroundColor({ color: color });
        } else {
            browser.browserAction.getBadgeText({}).then(function (oldText) {
                if (oldText !== text) {
                    browser.browserAction.setBadgeText({ text: text });
                    browser.browserAction.setBadgeBackgroundColor({ color: color });
                }
            });
        }
    });
}

function setBadge(source, text, color) {
    badges[source] = { text: text, color: color };
    updateBadge(text, color);
}

function clearBadge(source) {
    delete (badges[source]);
    var keys = Object.keys(badges);
    if (keys.length === 0) {
        updateBadge('', [0, 0, 0, 0]);
        return;
    }

    var last = badges[keys[keys.length - 1]];
    updateBadge(last.text, last.color);
}

function contextMenuHandler(info, tab) {
    postMessage({
        type: 'typeShare',
        data: {
            target: info.menuItemId,
            url: info.linkUrl || info.srcUrl || info.frameUrl || info.pageUrl,
        }
    });
}

function createContextMenus(devices) {
    if (disableContextMenu) {
        browser.contextMenus.removeAll();
        return;
    }
    browser.contextMenus.removeAll().then(function () {
        var devs = devices;
        if (defaultOnly && defaultDeviceId) {
            devs = {};
            devs[defaultDeviceId] = devices[defaultDeviceId];
        }
        var keys = Object.keys(devs);
        var active = false;
        keys.forEach(function (key) {
            if (devs[key] !== null || devs[key] !== undefined) {
                if (devs[key].isReachable && devs[key].isTrusted) {
                    active = true;
                    return;
                }
            }
        });
        if (!active) {
            setBadge('active', '!', orange);
            return;
        }
        if (keys.length === 0) {
            return;
        }
        clearBadge('active');

        if (keys.length === 1) {
            var key = keys[0];
            browser.contextMenus.create({
                id: key,
                title: 'KDE Connect (' + devs[key].name + ')',
                enabled: devs[key].isReachable && devs[key].isTrusted,
                contexts: ['page', 'frame', 'link', 'image', 'video', 'audio'],
                onclick: contextMenuHandler,
            });
            return;
        }

        browser.contextMenus.create({
            id: 'kdeconnectRoot',
            title: 'KDE Connect',
            contexts: ['page', 'frame', 'link', 'image', 'video', 'audio'],
        });
        Object.keys(devs).forEach(function (key) {
            browser.contextMenus.create({
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
    if (disableContextMenu) {
        return;
    }
    browser.contextMenus.update(device.id, {
        title: device.name,
        enabled: device.isReachable && device.isTrusted,
    });
}

function updateDevice(device) {
    var known = knownDevices[device.id];
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
    if (areaName !== 'sync') {
        return Promise.resolve();
    }
    var newDefaultDeviceId = changeValue(changes.defaultDeviceId);
    if (newDefaultDeviceId !== undefined) {
        defaultDeviceId = newDefaultDeviceId;
    }
    var newDefaultOnly = changeValue(changes.defaultOnly);
    if (newDefaultOnly !== undefined) {
        defaultOnly = newDefaultOnly;
    }
    var newDisableContextMenu = changeValue(changes.disableContextMenu);
    if (newDisableContextMenu !== undefined) {
        disableContextMenu = newDisableContextMenu;
    }
    createContextMenus(knownDevices);
    return Promise.resolve();
}

function restoreOptions() {
    browser.storage.onChanged.addListener(onStorageChanged);
    browser.storage.sync.get({
        defaultOnly: false,
        defaultDeviceId: null,
        disableContextMenu: false,
    }).then(function (items) {
        onStorageChanged({
            defaultDeviceId: {
                newValue: items.defaultDeviceId,
            },
            defaultOnly: {
                newValue: items.defaultOnly,
            },
            disableContextMenu: {
                newValue: items.disableContextMenu,
            },
        }, 'sync');
    });
}

function onPortMessage(msg) {
    switch (msg.type) {
        case 'typeDeviceUpdate':
            updateDevice(msg.data);
            sendMessage(msg);
            break;
        case 'typeDevices':
            knownDevices = msg.data;
            createContextMenus(msg.data);
            sendMessage(msg);
            break;
        case 'typeError':
            sendMessage({
                type: 'typeStatus',
                data: {
                    type: 'typeError',
                    key: 'host',
                    error: msg.data,
                }
            });
            break;
        case 'typeVersion':
            if (msg.data !== protocolVersion) {
                updatePending = protocolVersion;
                setBadge('update', '!', blue);
                sendMessage({
                    type: 'typeStatus',
                    data: {
                        type: 'typeVersion',
                        key: 'update',
                        update: protocolVersion,
                        current: msg.data,
                    }
                });
            } else {
                updatePending = false;
                clearBadge('update');
                sendMessage({ type: 'typeClearStatus', data: { key: 'update' } });
            }
            break;
        default:
            sendMessage(msg);
    }
    return Promise.resolve();
}

function resetReconnect() {
    reconnectDelay = 100;
}

function onDisconnect() {
    port = null;
    setBadge('connected', '!', red);
    sendMessage({
        type: 'typeStatus', data: {
            type: 'typeError',
            key: 'connected',
            error: 'could not connect to native host',
        }
    });
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
    if (browser.runtime.lastError) {
        message = browser.runtime.lastError.message;
    }
    console.warn('Disconnected from native host: ' + message);

    // Exponential back-off on reconnect
    reconnectTimer = window.setTimeout(function () {
        connect();
    }, reconnectDelay);
    reconnectDelay = reconnectDelay * 2;
    return Promise.resolve();
}

function versionTimeout() {
    if (versionTimer) {
        window.clearTimeout(versionTimer);
    }
    versionTimer = window.setTimeout(function () {
        if (updatePending === null) {
            // We did not receive a response to our typeUpdate message from the
            // host
            sendMessage({
                type: 'typeStatus', data: {
                    type: 'typeError',
                    key: 'update',
                    error: 'no version response received from native host',
                }
            });
            setBadge('update', '!', red);
        }
    }, 500);
}

function connect() {
    clearBadge('connected');
    sendMessage({ type: 'typeClearStatus', data: { key: 'connected' } });
    port = browser.runtime.connectNative(hostname);
    // Reset the back-off delay if we stay connected
    reconnectResetTimer = window.setTimeout(function () {
        reconnectDelay = 100;
    }, reconnectDelay * 0.9);

    port.onDisconnect.addListener(onDisconnect);
    port.onMessage.addListener(onPortMessage);
    port.postMessage({ type: 'typeVersion' });
    versionTimeout();
    port.postMessage({ type: 'typeDevices' });
}

connect();

restoreOptions();

browser.runtime.onMessage.addListener(onMessage);

browser.tabs.onActivated.addListener(function (info) {
    browser.tabs.get(info.tabId).then(toggleAction);
});

browser.tabs.onUpdated.addListener(function (tabId, change, tab) {
    toggleAction(tab);
});
