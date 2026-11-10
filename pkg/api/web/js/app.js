/**
 * Raspiducky Core Web Application Logic
 * Global State, Tablist Navigation, WebSocket Client & API Helper
 */

const state = {
    activeTab: 'gadget',
    ws: null,
    wsConnected: false,
    wsReconnectAttempts: 0,
    wsReconnectTimer: null,
    gadget: {
        deployed: false,
        activeFunctions: [],
        udc: '',
        maxEndpoints: 0,
        endpointCosts: {},
        config: {}
    },
    scripts: [],
    editorDelayDirty: false,
    lastTerminalJobId: null,
    jobStartTime: null,
    jobTimerInterval: null
};

document.addEventListener('DOMContentLoaded', async () => {
    initTabs();
    initGadgetControls();
    initEditorControls();
    initWebSocket();
    await Promise.all([loadGadgetStatus(), loadScriptsLibrary(), syncJobStatus()]);
});

function initTabs() {
    const tablist = document.querySelector('[role="tablist"]');
    const tabs = Array.from(tablist.querySelectorAll('[role="tab"]'));
    const panels = Array.from(document.querySelectorAll('[role="tabpanel"]'));

    for (const tab of tabs) {
        tab.addEventListener('click', () => switchTab(tab.dataset.tab));
    }

    tablist.addEventListener('keydown', (event) => {
        const currentIndex = tabs.indexOf(document.activeElement);
        if (currentIndex === -1) return;

        let nextIndex = -1;
        if (event.key === 'ArrowRight') {
            nextIndex = (currentIndex + 1) % tabs.length;
        } else if (event.key === 'ArrowLeft') {
            nextIndex = (currentIndex - 1 + tabs.length) % tabs.length;
        } else if (event.key === 'Home') {
            nextIndex = 0;
        } else if (event.key === 'End') {
            nextIndex = tabs.length - 1;
        }

        if (nextIndex !== -1) {
            event.preventDefault();
            tabs[nextIndex].focus();
            switchTab(tabs[nextIndex].dataset.tab);
        }
    });

    for (const panel of panels) {
        panel.addEventListener('beforematch', () => switchTab(panel.dataset.tab));
    }
}

function switchTab(tabId) {
    state.activeTab = tabId;

    const tabs = document.querySelectorAll('[role="tab"]');
    for (const tab of tabs) {
        const selected = tab.dataset.tab === tabId;
        tab.setAttribute('aria-selected', String(selected));
        tab.tabIndex = selected ? 0 : -1;
        if (selected) {
            tab.scrollIntoView({ block: 'nearest', inline: 'nearest', container: 'nearest' });
        }
    }

    const panels = document.querySelectorAll('[role="tabpanel"]');
    for (const panel of panels) {
        if (panel.dataset.tab === tabId) {
            panel.hidden = false;
        } else {
            panel.hidden = 'until-found';
        }
    }
}

function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws`;

    updateWSStatus('connecting');

    if (state.ws) {
        state.ws.onclose = null;
        state.ws.onerror = null;
        state.ws.onmessage = null;
        state.ws.onopen = null;
        state.ws.close();
        state.ws = null;
    }

    try {
        state.ws = new WebSocket(wsUrl);

        state.ws.onopen = () => {
            state.wsConnected = true;
            state.wsReconnectAttempts = 0;
            updateWSStatus('connected');
            appendLog('INFO', 'SYS', 'WebSocket connection established.');
            void syncJobStatus();
        };

        state.ws.onmessage = (event) => {
            try {
                const message = JSON.parse(event.data);
                handleWSMessage(message);
            } catch (err) {
                console.error('Failed to parse WS message:', err);
            }
        };

        state.ws.onclose = () => {
            state.wsConnected = false;
            updateWSStatus('disconnected');
            scheduleWSReconnect();
        };

        state.ws.onerror = () => {
            state.wsConnected = false;
            updateWSStatus('disconnected');
        };
    } catch (err) {
        console.error('WebSocket initialization error:', err);
        scheduleWSReconnect();
    }
}

function scheduleWSReconnect() {
    if (state.wsReconnectTimer) {
        clearTimeout(state.wsReconnectTimer);
    }
    state.wsReconnectAttempts++;
    const delay = Math.min(5000, 1000 * state.wsReconnectAttempts);
    state.wsReconnectTimer = setTimeout(initWebSocket, delay);
}

function updateWSStatus(status) {
    const dot = document.getElementById('ws-status-dot');
    const text = document.getElementById('ws-status-text');

    if (status === 'connected') {
        dot.className = 'status-dot connected';
        text.textContent = 'Live Connected';
    } else if (status === 'connecting') {
        dot.className = 'status-dot';
        text.textContent = 'Connecting...';
    } else {
        dot.className = 'status-dot disconnected';
        text.textContent = 'Disconnected (Retrying)';
    }
}

function handleWSMessage(msg) {
    switch (msg.type) {
        case 'log':
            appendLog(msg.level, msg.source, msg.message);
            break;
        case 'led_state':
            updateLEDIndicators(msg.payload);
            break;
        case 'gadget_status':
            updateGadgetUI(msg.payload);
            break;
        case 'job_status':
            updateJobStatusUI(msg.payload);
            break;
        default:
            break;
    }
}

function updateLEDIndicators(leds) {
    document.getElementById('led-num').classList.toggle('active', Boolean(leds.numLock));
    document.getElementById('led-caps').classList.toggle('active', Boolean(leds.capsLock));
    document.getElementById('led-scroll').classList.toggle('active', Boolean(leds.scrollLock));
}

async function apiCall(endpoint, method = 'GET', data = null) {
    const options = {
        method,
        headers: { 'Content-Type': 'application/json' }
    };
    if (data !== null) {
        options.body = JSON.stringify(data);
    }

    const response = await fetch(endpoint, options);
    if (!response.ok) {
        const errText = await response.text();
        throw new Error(`API ${method} ${endpoint} failed (${response.status}): ${errText.trim()}`);
    }

    if (response.status === 204) {
        return null;
    }
    return await response.json();
}
