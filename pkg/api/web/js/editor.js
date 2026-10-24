/**
 * Script Execution Engine, Code Editor, Console Terminal & Payload Library Manager
 */

// Line Numbers Sync
function syncLineNumbersScroll() {
    const editor = document.getElementById('code-editor');
    const lineNumbersElem = document.getElementById('line-numbers');
    lineNumbersElem.scrollTop = editor.scrollTop;
}

function handleEditorKeyDown(e) {
    if (e.key === 'Tab') {
        e.preventDefault();
        insertSnippet('    ');
    }
}

function updateLineNumbers() {
    const editor = document.getElementById('code-editor');
    const lineNumbersElem = document.getElementById('line-numbers');

    const lines = editor.value.split('\n').length;
    let numbersHtml = '';
    for (let i = 1; i <= lines; i++) {
        numbersHtml += `${i}<br>`;
    }
    lineNumbersElem.innerHTML = numbersHtml;
}

function insertSnippet(text) {
    const editor = document.getElementById('code-editor');
    const start = editor.selectionStart;
    const end = editor.selectionEnd;
    const val = editor.value;

    if (text.endsWith('\n') && start > 0 && val[start - 1] !== '\n') {
        text = '\n' + text;
    }

    editor.value = val.substring(0, start) + text + val.substring(end);
    editor.selectionStart = editor.selectionEnd = start + text.length;
    editor.focus();
    updateLineNumbers();
}

function onScriptFormatChange() {
    const formatSelect = document.getElementById('script-format');
    const titleInput = document.getElementById('script-title');

    const currentExt = formatSelect.value === 'javascript' ? '.js' : '.ducky';
    let baseName = titleInput.value.replace(/\.(ducky|txt|js)$/i, '');
    if (!baseName) baseName = 'untitled_payload';
    titleInput.value = baseName + currentExt;
}

// Script Execution Engine Controls
async function runScript() {
    const code = document.getElementById('code-editor').value;
    const name = document.getElementById('script-title').value.trim();
    const type = document.getElementById('script-format').value;
    const typingDelayMs = Number.parseInt(document.getElementById('editor-typing-delay').value, 10);

    if (!code.trim()) {
        appendLog('WARN', 'EDITOR', 'Cannot execute empty script.');
        return;
    }

    const runBtn = document.getElementById('btn-run-script');
    const stopBtn = document.getElementById('btn-stop-script');

    if (runBtn.disabled) return;
    runBtn.disabled = true;
    stopBtn.disabled = false;

    appendLog('INFO', 'RUNNER', `Triggering execution for payload: ${name} (${type})...`);

    try {
        const response = await apiCall('/api/run', 'POST', {
            script: code,
            name,
            type,
            typingDelayMs
        });

        if (response.jobId && state.lastTerminalJobId !== response.jobId) {
            updateJobStatusUI({
                id: response.jobId,
                name,
                status: 'RUNNING'
            });
        }
    } catch (err) {
        appendLog('ERROR', 'RUNNER', `Failed to launch script: ${err.message}`);
        runBtn.disabled = false;
        stopBtn.disabled = true;
    }
}

async function stopScript() {
    appendLog('WARN', 'RUNNER', 'Requesting emergency stop for active payload job...');

    try {
        await apiCall('/api/stop', 'POST', {});
        appendLog('INFO', 'RUNNER', 'Job stop signal dispatched successfully.');
        updateJobStatusUI({ status: 'STOPPED' });
    } catch (err) {
        appendLog('ERROR', 'RUNNER', `Failed to stop job: ${err.message}`);
    }
}

function clearJobTimer() {
    if (state.jobTimerInterval) {
        clearInterval(state.jobTimerInterval);
        state.jobTimerInterval = null;
    }
}

function updateJobStatusUI(job) {
    const jobBar = document.getElementById('job-status-bar');
    const pill = document.getElementById('job-status-pill');
    const idDisp = document.getElementById('job-id-display');
    const nameDisp = document.getElementById('job-name-display');
    const runBtn = document.getElementById('btn-run-script');
    const stopBtn = document.getElementById('btn-stop-script');

    const status = job.status.toUpperCase();
    const isTerminal = status === 'COMPLETED' || status === 'FAILED' || status === 'STOPPED';

    if (isTerminal) {
        if (job.id) {
            state.lastTerminalJobId = job.id;
        }
        clearJobTimer();
        jobBar.classList.add('hidden');
        runBtn.disabled = false;
        stopBtn.disabled = true;
        state.jobStartTime = null;
        return;
    }

    jobBar.classList.remove('hidden');
    pill.textContent = status;
    pill.className = `job-status-pill ${status.toLowerCase()}`;
    idDisp.textContent = `Job ID: ${job.id}`;
    nameDisp.textContent = `Script: ${job.name}`;

    runBtn.disabled = true;
    stopBtn.disabled = false;

    // Only start the timer once per job
    if (!state.jobStartTime) {
        state.jobStartTime = Date.now();
    }
    clearJobTimer();
    state.jobTimerInterval = setInterval(() => {
        const elapsed = ((Date.now() - state.jobStartTime) / 1000).toFixed(1);
        document.getElementById('job-timer-display').textContent = `Duration: ${elapsed}s`;
    }, 100);
}

// Live Console Output Terminal
function matchesLogFilter(level, source, filter) {
    if (filter === 'ALL') return true;
    if (filter === 'HID') return source === 'HID' || source === 'DUCKY' || source === 'JS';
    return level === filter;
}

function appendLog(level, source, message) {
    const terminal = document.getElementById('terminal-output');
    const currentFilter = document.getElementById('log-filter').value;
    const normLevel = level.toUpperCase();
    const normSource = source.toUpperCase();
    const timeStr = new Date().toLocaleTimeString('en-US', { hour12: false });
    const tagClass = `tag-${normLevel.toLowerCase()}`;

    const entry = document.createElement('div');
    entry.className = `log-entry log-${normLevel.toLowerCase()}`;
    entry.dataset.level = normLevel;
    entry.dataset.source = normSource;
    entry.style.display = matchesLogFilter(normLevel, normSource, currentFilter) ? 'flex' : 'none';
    entry.innerHTML = `
        <span class="log-time">[${timeStr}]</span>
        <span class="log-tag ${tagClass}">${source}</span>
        <span class="log-msg">${escapeHtml(message)}</span>
    `;

    terminal.appendChild(entry);
    while (terminal.childElementCount > 500) {
        terminal.firstElementChild.remove();
    }
    terminal.scrollTop = terminal.scrollHeight;
}

function clearConsole() {
    const terminal = document.getElementById('terminal-output');
    terminal.innerHTML = `
        <div class="log-entry log-system" data-level="INFO" data-source="SYS">
            <span class="log-time">[${new Date().toLocaleTimeString('en-US', { hour12: false })}]</span>
            <span class="log-tag tag-info">SYS</span>
            <span class="log-msg">Console cleared.</span>
        </div>
    `;
    filterTerminalLogs();
}

function filterTerminalLogs() {
    const filter = document.getElementById('log-filter').value;
    const entries = document.querySelectorAll('.terminal-body .log-entry');

    entries.forEach(entry => {
        const visible = matchesLogFilter(entry.dataset.level, entry.dataset.source, filter);
        entry.style.display = visible ? 'flex' : 'none';
    });
}

function escapeHtml(str) {
    return String(str)
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}

// Payload Library Management
async function loadScriptsLibrary() {
    try {
        const scripts = await apiCall('/api/scripts');
        if (Array.isArray(scripts)) {
            state.scripts = scripts;
            renderLibrary();
        }
    } catch (err) {
        appendLog('ERROR', 'SYSTEM', `Failed to load script library: ${err.message}`);
    }
}

function renderLibrary() {
    const grid = document.getElementById('library-grid');
    const query = document.getElementById('library-search-input').value.toLowerCase().trim();
    const filtered = state.scripts.filter(s =>
        s.name.toLowerCase().includes(query) ||
        s.description.toLowerCase().includes(query)
    );

    if (filtered.length === 0) {
        grid.innerHTML = `
            <div class="glass-card" style="grid-column: 1 / -1; text-align: center; color: var(--text-muted);">
                <p>No saved payloads found in library.</p>
            </div>
        `;
        return;
    }

    grid.innerHTML = filtered.map(s => {
        const badgeClass = s.type === 'javascript' ? 'badge-js' : 'badge-ducky';
        const typeLabel = s.type === 'javascript' ? 'JavaScript' : 'DuckyScript';
        const safeName = encodeURIComponent(s.name);

        return `
            <div class="script-card">
                <div>
                    <div class="script-card-header">
                        <span class="script-card-title">${escapeHtml(s.name)}</span>
                        <span class="script-badge ${badgeClass}">${typeLabel}</span>
                    </div>
                    <p class="script-card-desc">${escapeHtml(s.description)}</p>
                    <div class="script-code-preview">${escapeHtml(s.content)}</div>
                </div>
                <div class="script-card-actions">
                    <button class="btn btn-xs btn-primary" onclick="loadScriptToEditor(decodeURIComponent('${safeName}'))">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                        Load in Editor
                    </button>
                    <button class="btn btn-xs btn-success" onclick="quickRunScript(decodeURIComponent('${safeName}'))">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                        Quick Run
                    </button>
                    <button class="btn btn-xs btn-danger" onclick="deleteScriptFromLibrary(decodeURIComponent('${safeName}'))">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                    </button>
                </div>
            </div>
        `;
    }).join('');
}

function loadScriptToEditor(name) {
    const s = state.scripts.find(item => item.name === name);
    if (!s) return;

    document.getElementById('script-title').value = s.name;
    document.getElementById('script-format').value = s.type;
    document.getElementById('code-editor').value = s.content;
    updateLineNumbers();
    switchTab('editor');
    appendLog('INFO', 'LIBRARY', `Loaded payload [${s.name}] into editor.`);
}

async function quickRunScript(name) {
    if (!state.scripts.some(item => item.name === name)) return;

    loadScriptToEditor(name);
    await runScript();
}

async function saveCurrentScript() {
    const titleInput = document.getElementById('script-title');
    const name = titleInput.value.trim();
    const type = document.getElementById('script-format').value;
    const content = document.getElementById('code-editor').value;

    if (!name || !content.trim()) {
        appendLog('WARN', 'LIBRARY', 'Specify a valid name and non-empty content before saving.');
        return;
    }

    const payload = {
        name,
        type,
        content,
        description: `Saved payload (${type})`
    };

    try {
        const res = await apiCall('/api/scripts', 'POST', payload);
        if (res?.name) {
            titleInput.value = res.name;
        }
        appendLog('INFO', 'LIBRARY', `Payload [${titleInput.value}] saved to library successfully!`);
        await loadScriptsLibrary();
    } catch (err) {
        appendLog('ERROR', 'LIBRARY', `Failed to save payload: ${err.message}`);
    }
}

async function deleteScriptFromLibrary(name) {
    if (!confirm(`Are you sure you want to delete script "${name}"?`)) return;

    try {
        await apiCall(`/api/scripts/${encodeURIComponent(name)}`, 'DELETE');
        appendLog('INFO', 'LIBRARY', `Payload [${name}] deleted from library.`);
        await loadScriptsLibrary();
    } catch (err) {
        appendLog('ERROR', 'LIBRARY', `Failed to delete payload: ${err.message}`);
    }
}

function openNewScriptModal() {
    document.getElementById('script-title').value = 'new_payload.ducky';
    document.getElementById('script-format').value = 'duckyscript';
    document.getElementById('code-editor').value = 'REM New Raspiducky Payload\nDELAY 1000\n';
    updateLineNumbers();
    switchTab('editor');
}
