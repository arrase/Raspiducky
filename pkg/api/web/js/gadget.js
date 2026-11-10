/**
 * USB Gadget Status & Controls Handler
 */

const GADGET_TOGGLES = [
    { id: 'gadget-keyboard', key: 'keyboard' },
    { id: 'gadget-mouse', key: 'mouse' },
    { id: 'gadget-storage', key: 'storage' },
    { id: 'gadget-rndis', key: 'rndis' },
    { id: 'gadget-ecm', key: 'ecm' },
    { id: 'gadget-serial', key: 'serial' }
];

function initGadgetControls() {
    document.getElementById('gadget-config-form').addEventListener('submit', applyGadgetConfig);
    document.getElementById('btn-reload-gadget').addEventListener('click', loadGadgetStatus);

    for (const { id } of GADGET_TOGGLES) {
        document.getElementById(id).addEventListener('change', updateEndpointMonitor);
    }
}

async function loadGadgetStatus() {
    try {
        const data = await apiCall('/api/gadget');
        if (data) {
            updateGadgetUI(data);
            appendLog('INFO', 'GADGET', 'Loaded gadget status from USB configfs.');
        }
    } catch (err) {
        appendLog('ERROR', 'GADGET', `Failed to load gadget status: ${err.message}`);
    }
}

function populateGadgetToggles(cfg) {
    for (const { id, key } of GADGET_TOGGLES) {
        document.getElementById(id).checked = Boolean(cfg[key]);
    }
    document.getElementById('storage-cdrom').checked = Boolean(cfg.storageCdrom);
    document.getElementById('storage-readonly').checked = Boolean(cfg.storageReadOnly);
}

function populateGadgetForm(cfg, forceSyncEditorDelay = false) {
    const fields = [
        ['vendor-id', cfg.vendorId],
        ['product-id', cfg.productId],
        ['manufacturer-name', cfg.manufacturer],
        ['product-name', cfg.product],
        ['serial-number', cfg.serialNumber],
        ['keyboard-layout', cfg.keyboardLayout],
        ['storage-size', cfg.storageSizeMb],
        ['typing-delay', cfg.typingDelayMs]
    ];
    for (const [id, value] of fields) {
        if (value !== undefined && value !== null) {
            document.getElementById(id).value = value;
        }
    }
    if (cfg.typingDelayMs !== undefined && cfg.typingDelayMs !== null && (!state.editorDelayDirty || forceSyncEditorDelay)) {
        document.getElementById('editor-typing-delay').value = cfg.typingDelayMs;
    }
}

function updateGadgetUI(gadgetData, forceSyncEditorDelay = false) {
    state.gadget = { ...state.gadget, ...gadgetData };
    const cfg = state.gadget.config;

    document.getElementById('udc-name').textContent = state.gadget.udc || 'Inactive';

    populateGadgetToggles(cfg);
    populateGadgetForm(cfg, forceSyncEditorDelay);
    updateEndpointMonitor();
}

function countActiveEndpoints() {
    const costs = state.gadget.endpointCosts;
    let total = 0;
    let anyChecked = false;
    for (const { id, key } of GADGET_TOGGLES) {
        if (document.getElementById(id).checked) {
            anyChecked = true;
            total += costs[key] || 0;
        }
    }
    if (anyChecked) {
        total += costs.ep0 || 1;
    }
    return total;
}

function updateEndpointMonitor() {
    const endpoints = countActiveEndpoints();
    const maxEndpoints = state.gadget.maxEndpoints;
    const countElem = document.getElementById('endpoint-count');
    const progressElem = document.getElementById('endpoint-progress');
    const warningElem = document.getElementById('endpoint-warning');
    const applyBtn = document.getElementById('btn-apply-gadget');

    if (endpoints === 0) {
        countElem.textContent = maxEndpoints > 0 ? `0 / ${maxEndpoints}` : '0 / ?';
        progressElem.style.width = '0%';
        progressElem.style.backgroundColor = 'var(--accent-red)';
        warningElem.textContent = 'Select at least one USB function to deploy.';
        warningElem.style.display = 'block';
        applyBtn.disabled = true;
        return;
    }

    if (maxEndpoints <= 0) {
        countElem.textContent = `${endpoints} / ?`;
        progressElem.style.width = '50%';
        progressElem.style.backgroundColor = 'var(--accent-cyan)';
        warningElem.textContent = '';
        warningElem.style.display = 'none';
        applyBtn.disabled = false;
        return;
    }

    const exceeded = endpoints > maxEndpoints;
    const pct = Math.min((endpoints / maxEndpoints) * 100, 100);

    countElem.textContent = `${endpoints} / ${maxEndpoints}`;
    progressElem.style.width = `${pct}%`;
    progressElem.style.backgroundColor = exceeded ? 'var(--accent-red)' : 'var(--accent-cyan)';
    warningElem.textContent = exceeded
        ? `Hardware limit exceeded. The USB controller supports a maximum of ${maxEndpoints} endpoints.`
        : '';
    warningElem.style.display = exceeded ? 'block' : 'none';
    applyBtn.disabled = exceeded;
}

function setTogglesDisabled(disabled) {
    for (const { id } of GADGET_TOGGLES) {
        document.getElementById(id).disabled = disabled;
    }
}

async function applyGadgetConfig(event) {
    event.preventDefault();

    document.getElementById('btn-apply-gadget').disabled = true;
    setTogglesDisabled(true);

    const payload = {
        vendorId: document.getElementById('vendor-id').value.trim(),
        productId: document.getElementById('product-id').value.trim(),
        manufacturer: document.getElementById('manufacturer-name').value.trim(),
        product: document.getElementById('product-name').value.trim(),
        serialNumber: document.getElementById('serial-number').value.trim(),
        keyboardLayout: document.getElementById('keyboard-layout').value.trim(),
        storageSizeMb: Number.parseInt(document.getElementById('storage-size').value, 10),
        typingDelayMs: Number.parseInt(document.getElementById('typing-delay').value, 10),
        storageCdrom: document.getElementById('storage-cdrom').checked,
        storageReadOnly: document.getElementById('storage-readonly').checked
    };
    for (const { id, key } of GADGET_TOGGLES) {
        payload[key] = document.getElementById(id).checked;
    }

    try {
        appendLog('INFO', 'GADGET', `Deploying gadget config (VID: ${payload.vendorId}, PID: ${payload.productId})...`);
        const result = await apiCall('/api/gadget', 'POST', payload);
        state.editorDelayDirty = false;
        if (result) {
            updateGadgetUI(result, true);
        }
        appendLog('INFO', 'GADGET', 'USB Gadget configuration successfully deployed.');
    } catch (err) {
        populateGadgetToggles(state.gadget.config);
        appendLog('ERROR', 'GADGET', `Failed to deploy USB gadget config: ${err.message}`);
    } finally {
        setTogglesDisabled(false);
        updateEndpointMonitor();
    }
}
