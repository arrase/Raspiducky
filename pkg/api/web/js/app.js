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
        keyboard: true,
        mouse: true,
        storage: false,
        ethernet: false,
        serial: false,
        vendorId: '0x1d6b',
        productId: '0x0104',
        manufacturer: 'Raspiducky Labs',
        product: 'Raspiducky Multi-Function HID',
        serialNumber: 'RPD-2026-0001',
        udc: '3f980000.usb',
        deployed: true
    },
    scripts: [],
    currentJob: null,
    jobStartTime: null,
    jobTimerInterval: null,
    logs: []
};

document.addEventListener('DOMContentLoaded', () => {
    initWebSocket();
    loadGadgetStatus();
    loadScriptsLibrary();
    updateLineNumbers();
});

// Tab Switching
function switchTab(tabId) {
    state.activeTab = tabId;
    
    // Update nav tab buttons
    document.querySelectorAll('.nav-tab').forEach(tab => tab.classList.remove('active'));
    const btn = document.getElementById(`tab-btn-${tabId}`);
    if (btn) btn.classList.add('active');

    // Update tab content views
    document.querySelectorAll('.tab-content').forEach(view => view.classList.remove('active'));
    const view = document.getElementById(`view-${tabId}`);
    if (view) view.classList.add('active');
}

// WebSocket Connection Manager
function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws`;

    updateWSStatus('connecting');

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
    state.wsReconnectAttempts++;
    const delay = Math.min(5000, 1000 * state.wsReconnectAttempts);
    setTimeout(initWebSocket, delay);
}

function updateWSStatus(status) {
    const dot = document.getElementById('ws-status-dot');
    const text = document.getElementById('ws-status-text');

    if (!dot || !text) return;

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
            appendLog(msg.level || 'INFO', msg.source || 'PAYLOAD', msg.message);
            break;
        case 'led_state':
            updateLEDIndicators(msg.payload || msg);
            break;
        case 'gadget_status':
            updateGadgetUI(msg.payload || msg);
            break;
        case 'job_status':
            updateJobStatusUI(msg.payload || msg);
            break;
        default:
            console.log('Unhandled WS message type:', msg.type, msg);
    }
}

// LED Indicators Update
function updateLEDIndicators(leds) {
    const num = document.getElementById('led-num');
    const caps = document.getElementById('led-caps');
    const scroll = document.getElementById('led-scroll');

    if (num) num.classList.toggle('active', !!leds.numLock);
    if (caps) caps.classList.toggle('active', !!leds.capsLock);
    if (scroll) scroll.classList.toggle('active', !!leds.scrollLock);
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
