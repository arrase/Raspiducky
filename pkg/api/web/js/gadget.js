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

function updateGadgetUI(gadgetData) {
    if (gadgetData) {
        state.gadget = { ...state.gadget, ...gadgetData };
    }
    const cfg = state.gadget.config || state.gadget;

    // UDC Name Badge
    const udcElem = document.getElementById('udc-name');
    if (udcElem) udcElem.textContent = state.gadget.udc || 'Inactive';

    // Toggles
    const kb = document.getElementById('gadget-keyboard');
    const ms = document.getElementById('gadget-mouse');
    const st = document.getElementById('gadget-storage');
    const eth = document.getElementById('gadget-ethernet');
    const ser = document.getElementById('gadget-serial');

    if (kb) kb.checked = !!cfg.keyboard;
    if (ms) ms.checked = !!cfg.mouse;
    if (st) st.checked = !!cfg.storage;
    if (eth) eth.checked = !!cfg.ethernet;
    if (ser) ser.checked = !!cfg.serial;

    // Form inputs
    const vid = document.getElementById('vendor-id');
    const pid = document.getElementById('product-id');
    const mfg = document.getElementById('manufacturer-name');
    const prod = document.getElementById('product-name');
    const sn = document.getElementById('serial-number');
    const layout = document.getElementById('keyboard-layout');
    const storageSize = document.getElementById('storage-size');

    if (vid && cfg.vendorId) vid.value = cfg.vendorId;
    if (pid && cfg.productId) pid.value = cfg.productId;
    if (mfg && cfg.manufacturer) mfg.value = cfg.manufacturer;
    if (prod && cfg.product) prod.value = cfg.product;
    if (sn && cfg.serialNumber) sn.value = cfg.serialNumber;
    if (layout && cfg.keyboardLayout) layout.value = cfg.keyboardLayout;
    if (storageSize && cfg.storageSizeMb) storageSize.value = cfg.storageSizeMb;

    updateEndpointMonitor();
}

function updateEndpointMonitor() {
    let endpoints = 0;
    const kb = document.getElementById('gadget-keyboard');
    const ms = document.getElementById('gadget-mouse');
    const st = document.getElementById('gadget-storage');
    const eth = document.getElementById('gadget-ethernet');
    const ser = document.getElementById('gadget-serial');

    if (kb && kb.checked) endpoints += 1;
    if (ms && ms.checked) endpoints += 1;
    if (st && st.checked) endpoints += 1;
    if (eth && eth.checked) endpoints += 4; // RNDIS (2) + ECM (2)
    if (ser && ser.checked) endpoints += 2; // ACM (2)

    const maxEndpoints = state.gadget.maxEndpoints || 0;
    const countElem = document.getElementById('endpoint-count');
    const progressElem = document.getElementById('endpoint-progress');
    const warningElem = document.getElementById('endpoint-warning');
    const applyBtn = document.getElementById('btn-apply-gadget');

    if (maxEndpoints === 0) {
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
        return;
    }

    if (countElem) countElem.textContent = `${endpoints} / ${maxEndpoints}`;
    if (progressElem) {
        const pct = Math.min((endpoints / maxEndpoints) * 100, 100);
        progressElem.style.width = `${pct}%`;
        progressElem.style.background = endpoints > maxEndpoints ? 'var(--accent-red)' : 'var(--accent-cyan)';
    }

    if (endpoints > maxEndpoints) {
        if (warningElem) {
            warningElem.textContent = `Hardware limit exceeded. The USB controller supports a maximum of ${maxEndpoints} IN endpoints.`;
            warningElem.style.display = 'block';
        }
        if (applyBtn) applyBtn.disabled = true;
    } else {
        if (warningElem) warningElem.style.display = 'none';
        if (applyBtn) applyBtn.disabled = false;
    }
}

// Add event listeners when DOM loads
document.addEventListener('DOMContentLoaded', () => {
    ['gadget-keyboard', 'gadget-mouse', 'gadget-storage', 'gadget-ethernet', 'gadget-serial'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.addEventListener('change', updateEndpointMonitor);
    });
});

async function applyGadgetConfig(event) {
    if (event) event.preventDefault();

    const applyBtn = document.getElementById('btn-apply-gadget');
    if (applyBtn) applyBtn.disabled = true;

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
        storageSizeMb: parseInt(document.getElementById('storage-size').value, 10) || 100
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
    }
}
