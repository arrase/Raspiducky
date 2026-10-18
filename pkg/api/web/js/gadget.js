/**
 * USB Gadget Status & Controls Handler
 */

const GADGET_TOGGLES = [
    { id: 'gadget-keyboard', key: 'keyboard', endpoints: 1 },
    { id: 'gadget-mouse', key: 'mouse', endpoints: 1 },
    { id: 'gadget-storage', key: 'storage', endpoints: 1 },
    { id: 'gadget-ethernet', key: 'ethernet', endpoints: 4 },
    { id: 'gadget-serial', key: 'serial', endpoints: 2 }
];

async function loadGadgetStatus() {
    try {
        const data = await apiCall('/api/gadget');
        if (data) {
            updateGadgetUI(data);
            appendLog('INFO', 'GADGET', 'Loaded gadget status from USB configfs');
        }
    } catch (err) {
        appendLog('ERROR', 'GADGET', `Failed to load gadget status: ${err.message}`);
    }
}

function populateGadgetToggles(cfg) {
    for (const { id, key } of GADGET_TOGGLES) {
        document.getElementById(id).checked = Boolean(cfg[key]);
    }
}

function populateGadgetForm(cfg) {
    const fields = [
        ['vendor-id', cfg.vendorId],
        ['product-id', cfg.productId],
        ['manufacturer-name', cfg.manufacturer],
        ['product-name', cfg.product],
        ['serial-number', cfg.serialNumber],
        ['keyboard-layout', cfg.keyboardLayout],
        ['storage-size', cfg.storageSizeMb],
    ];
    for (const [id, value] of fields) {
        if (value !== undefined && value !== null) {
            document.getElementById(id).value = value;
        }
    }
}

function updateGadgetUI(gadgetData) {
    state.gadget = { ...state.gadget, ...gadgetData };
    const cfg = state.gadget.config;

    document.getElementById('udc-name').textContent = state.gadget.udc || 'Inactive';

    populateGadgetToggles(cfg);
    populateGadgetForm(cfg);
    updateEndpointMonitor();
}

function countActiveEndpoints() {
    let total = 0;
    for (const { id, endpoints } of GADGET_TOGGLES) {
        if (document.getElementById(id).checked) {
            total += endpoints;
        }
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

    if (maxEndpoints <= 0) {
        countElem.textContent = `${endpoints} / ?`;
        progressElem.style.width = '100%';
        progressElem.style.background = 'var(--accent-red)';
        warningElem.textContent = 'Hardware limit unknown or debugfs not mounted. Deployment blocked for safety.';
        warningElem.style.display = 'block';
        applyBtn.disabled = true;
        return;
    }

    const exceeded = endpoints > maxEndpoints;
    const pct = Math.min((endpoints / maxEndpoints) * 100, 100);

    countElem.textContent = `${endpoints} / ${maxEndpoints}`;
    progressElem.style.width = `${pct}%`;
    progressElem.style.background = exceeded ? 'var(--accent-red)' : 'var(--accent-cyan)';
    warningElem.textContent = exceeded
        ? `Hardware limit exceeded. The USB controller supports a maximum of ${maxEndpoints} IN endpoints.`
        : '';
    warningElem.style.display = exceeded ? 'block' : 'none';
    applyBtn.disabled = exceeded;
}

// Add event listeners when DOM loads
document.addEventListener('DOMContentLoaded', () => {
    const applyBtn = document.getElementById('btn-apply-gadget');
    for (const { id } of GADGET_TOGGLES) {
        document.getElementById(id).addEventListener('change', async (e) => {
            updateEndpointMonitor();
            if (applyBtn.disabled) {
                e.target.checked = !e.target.checked;
                updateEndpointMonitor();
                return;
            }
            await applyGadgetConfig();
        });
    }
});

function setTogglesDisabled(disabled) {
    for (const { id } of GADGET_TOGGLES) {
        document.getElementById(id).disabled = disabled;
    }
}

async function applyGadgetConfig(event) {
    if (event) event.preventDefault();

    document.getElementById('btn-apply-gadget').disabled = true;
    setTogglesDisabled(true);

    const payload = {
        vendorId: document.getElementById('vendor-id').value.trim(),
        productId: document.getElementById('product-id').value.trim(),
        manufacturer: document.getElementById('manufacturer-name').value.trim(),
        product: document.getElementById('product-name').value.trim(),
        serialNumber: document.getElementById('serial-number').value.trim(),
        keyboardLayout: document.getElementById('keyboard-layout').value.trim(),
        storageSizeMb: Number.parseInt(document.getElementById('storage-size').value, 10)
    };
    for (const { id, key } of GADGET_TOGGLES) {
        payload[key] = document.getElementById(id).checked;
    }

    try {
        appendLog('INFO', 'GADGET', `Deploying gadget config (VID: ${payload.vendorId}, PID: ${payload.productId})...`);
        const result = await apiCall('/api/gadget', 'POST', payload);
        if (result) {
            updateGadgetUI(result);
        }
        appendLog('INFO', 'GADGET', 'USB Gadget configuration successfully deployed!');
    } catch (err) {
        populateGadgetToggles(state.gadget.config);
        appendLog('ERROR', 'GADGET', `Failed to deploy USB gadget config: ${err.message}`);
    } finally {
        setTogglesDisabled(false);
        updateEndpointMonitor();
    }
}
