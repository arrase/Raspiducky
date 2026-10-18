/**
 * Raspiducky Core Web Application Logic
 * Global State, Navigation, WebSocket Client & System Events
 */

const state = {
    activeTab: 'gadget',
    ws: null,
    wsConnected: false,
    wsReconnectAttempts: 0,
    gadget: {
        deployed: false,
        activeFunctions: [],
        udc: '',
        maxEndpoints: 0,
        config: {}
    },
    scripts: [],
    lastTerminalJobId: null,
    jobStartTime: null,
    jobTimerInterval: null,
    wsReconnectTimer: null
};

document.addEventListener('DOMContentLoaded', async () => {
    initWebSocket();
    updateLineNumbers();
    await Promise.all([loadGadgetStatus(), loadScriptsLibrary()]);
});

// Tab Switching
function switchTab(tabId) {
    state.activeTab = tabId;

    // Update nav tab buttons
    document.querySelectorAll('.nav-tab').forEach(tab => tab.classList.remove('active'));
    document.getElementById(`tab-btn-${tabId}`).classList.add('active');

    // Update tab content views
    document.querySelectorAll('.tab-content').forEach(view => view.classList.remove('active'));
    document.getElementById(`view-${tabId}`).classList.add('active');
}

// WebSocket Connection Manager
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
            appendLog('INFO', 'SYS', 'WebSocket Connection Established.');
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

        state.ws.onerror = (err) => {
            console.warn('WebSocket error:', err);
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

// Handle Incoming WebSocket Messages
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
            console.log('Unhandled WS message type:', msg.type, msg);
    }
}

// LED Indicators Update
function updateLEDIndicators(leds) {
    document.getElementById('led-num').classList.toggle('active', Boolean(leds.numLock));
    document.getElementById('led-caps').classList.toggle('active', Boolean(leds.capsLock));
    document.getElementById('led-scroll').classList.toggle('active', Boolean(leds.scrollLock));
}

// API Utility Helper
async function apiCall(endpoint, method = 'GET', data = null) {
    const options = {
        method,
        headers: { 'Content-Type': 'application/json' }
    };
    if (data) options.body = JSON.stringify(data);

    const response = await fetch(endpoint, options);
    if (!response.ok) {
        const errText = await response.text();
        throw new Error(`API ${method} ${endpoint} failed (${response.status}): ${errText}`);
    }
    
    if (response.status === 204) return null;
    return await response.json();
}
