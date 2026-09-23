/**
 * USB Gadget Status & Controls Handler
 */

async function loadGadgetStatus() {
    try {
        const data = await apiCall('/api/gadget');
        if (data) {
            updateGadgetUI(data);
            appendLog('INFO', 'GADGET', 'Loaded gadget status from USB configfs');
        }
    } catch (err) {
        console.warn('Could not fetch gadget status:', err);
        // Fallback to local default state for UI display
        updateGadgetUI(state.gadget);
    }
}

function populateGadgetToggles(cfg) {
    const toggles = [
        ['gadget-keyboard', !!cfg.keyboard],
        ['gadget-mouse', !!cfg.mouse],
        ['gadget-storage', !!cfg.storage],
        ['gadget-ethernet', !!cfg.ethernet],
        ['gadget-serial', !!cfg.serial],
    ];
    for (const [id, checked] of toggles) {
        const el = document.getElementById(id);
        if (el) el.checked = checked;
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
        const el = document.getElementById(id);
        if (el && value) el.value = value;
    }
}

function updateGadgetUI(gadgetData) {
    if (gadgetData) {
        state.gadget = { ...state.gadget, ...gadgetData };
    }
    const cfg = state.gadget.config || state.gadget;

    // UDC Name Badge
    const udcElem = document.getElementById('udc-name');
    if (udcElem) udcElem.textContent = state.gadget.udc || 'Inactive';

    populateGadgetToggles(cfg);
    populateGadgetForm(cfg);
    updateEndpointMonitor();
}

function countActiveEndpoints() {
    const costs = {
        'gadget-keyboard': 1,
        'gadget-mouse': 1,
        'gadget-storage': 1,
        'gadget-ethernet': 4,
        'gadget-serial': 2,
    };
    let total = 0;
    for (const [id, count] of Object.entries(costs)) {
        if (document.getElementById(id)?.checked) total += count;
    }
    return total;
}

function renderUnknownHardwareLimits(endpoints) {
    const countElem = document.getElementById('endpoint-count');
    const progressElem = document.getElementById('endpoint-progress');
    const warningElem = document.getElementById('endpoint-warning');
    const applyBtn = document.getElementById('btn-apply-gadget');

    if (countElem) countElem.textContent = `${endpoints} / ?`;
    if (progressElem) {
        progressElem.style.width = '100%';
        progressElem.style.background = 'var(--accent-red)';
    }
    if (warningElem) {
        warningElem.textContent = 'Hardware limit unknown or debugfs not mounted. Deployment blocked for safety.';
        warningElem.style.display = 'block';
    }
    if (applyBtn) applyBtn.disabled = true;
}

function renderHardwareLimits(endpoints, maxEndpoints) {
    const countElem = document.getElementById('endpoint-count');
    const progressElem = document.getElementById('endpoint-progress');
    const warningElem = document.getElementById('endpoint-warning');
    const applyBtn = document.getElementById('btn-apply-gadget');

    const exceeded = endpoints > maxEndpoints;
    if (countElem) countElem.textContent = `${endpoints} / ${maxEndpoints}`;
    if (progressElem) {
        const pct = Math.min((endpoints / maxEndpoints) * 100, 100);
        progressElem.style.width = `${pct}%`;
        progressElem.style.background = exceeded ? 'var(--accent-red)' : 'var(--accent-cyan)';
    }

    if (warningElem) {
        warningElem.textContent = exceeded ? `Hardware limit exceeded. The USB controller supports a maximum of ${maxEndpoints} IN endpoints.` : '';
        warningElem.style.display = exceeded ? 'block' : 'none';
    }
    if (applyBtn) applyBtn.disabled = exceeded;
}

function updateEndpointMonitor() {
    const endpoints = countActiveEndpoints();
    const maxEndpoints = state.gadget.maxEndpoints || 0;

    if (maxEndpoints === 0) {
        renderUnknownHardwareLimits(endpoints);
        return;
    }

    renderHardwareLimits(endpoints, maxEndpoints);
}

// Add event listeners when DOM loads
document.addEventListener('DOMContentLoaded', () => {
    ['gadget-keyboard', 'gadget-mouse', 'gadget-storage', 'gadget-ethernet', 'gadget-serial'].forEach(id => {
        const el = document.getElementById(id);
        if (el) {
            el.addEventListener('change', async (e) => {
                updateEndpointMonitor();
                const applyBtn = document.getElementById('btn-apply-gadget');
                if (applyBtn?.disabled) {
                    // Revert toggle if hardware limit exceeded
                    e.target.checked = !e.target.checked;
                    updateEndpointMonitor();
                    return;
                }
                await applyGadgetConfig();
            });
        }
    });
});

function setTogglesDisabled(disabled) {
    ['gadget-keyboard', 'gadget-mouse', 'gadget-storage', 'gadget-ethernet', 'gadget-serial'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.disabled = disabled;
    });
}

async function applyGadgetConfig(event) {
    if (event) event.preventDefault();

    const applyBtn = document.getElementById('btn-apply-gadget');
    if (applyBtn) applyBtn.disabled = true;
    setTogglesDisabled(true);

    const payload = {
        keyboard: document.getElementById('gadget-keyboard').checked,
        mouse: document.getElementById('gadget-mouse').checked,
        storage: document.getElementById('gadget-storage').checked,
        ethernet: document.getElementById('gadget-ethernet').checked,
        serial: document.getElementById('gadget-serial').checked,
        vendorId: document.getElementById('vendor-id').value.trim(),
        productId: document.getElementById('product-id').value.trim(),
        manufacturer: document.getElementById('manufacturer-name').value.trim(),
        product: document.getElementById('product-name').value.trim(),
        serialNumber: document.getElementById('serial-number').value.trim(),
        keyboardLayout: document.getElementById('keyboard-layout').value.trim(),
        storageSizeMb: Number.parseInt(document.getElementById('storage-size').value, 10) || 100
    };

    try {
        appendLog('INFO', 'GADGET', `Deploying gadget config (VID: ${payload.vendorId}, PID: ${payload.productId})...`);
        const result = await apiCall('/api/gadget', 'POST', payload);
        if (result) {
            updateGadgetUI(result);
        }
        appendLog('INFO', 'GADGET', 'USB Gadget configuration successfully deployed!');
    } catch (err) {
        appendLog('ERROR', 'GADGET', `Failed to deploy USB gadget config: ${err.message}`);
    } finally {
        if (applyBtn) applyBtn.disabled = false;
        setTogglesDisabled(false);
    }
}
