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
}

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
