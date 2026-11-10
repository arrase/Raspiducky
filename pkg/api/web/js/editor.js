/**
 * Script Execution Engine, Code Editor, Console Terminal & Payload Library Manager
 */

const SNIPPETS = {
    duckyscript: {
        delay: 'DELAY 500\n',
        string: 'STRING Hello World\n',
        'gui-r': 'GUI r\n',
        enter: 'ENTER\n'
    },
    javascript: {
        delay: 'delay(500);\n',
        string: 'type("Hello World");\n',
        'gui-r': 'press("GUI r");\n',
        enter: 'press("ENTER");\n'
    }
};

function initEditorControls() {
    const editor = document.getElementById('code-editor');
    editor.addEventListener('input', updateLineNumbers);
    editor.addEventListener('scroll', syncLineNumbersScroll);
    editor.addEventListener('keydown', handleEditorKeyDown);

    document.getElementById('script-format').addEventListener('change', onScriptFormatChange);
    document.getElementById('editor-typing-delay').addEventListener('input', () => {
        state.editorDelayDirty = true;
    });

    document.getElementById('btn-run-script').addEventListener('click', runScript);
    document.getElementById('btn-stop-script').addEventListener('click', stopScript);
    document.getElementById('btn-save-script').addEventListener('click', saveCurrentScript);
    document.getElementById('btn-clear-console').addEventListener('click', clearConsole);
    document.getElementById('log-filter').addEventListener('change', filterTerminalLogs);

    document.getElementById('editor-quick-snippets').addEventListener('click', (event) => {
        const chip = event.target.closest('[data-snippet]');
        if (!chip) return;
        const format = document.getElementById('script-format').value;
        const snippetSet = SNIPPETS[format];
        const snippet = snippetSet[chip.dataset.snippet];
        if (snippet) {
            insertSnippet(snippet);
        }
    });

    document.getElementById('library-search-input').addEventListener('input', renderLibrary);
    document.getElementById('btn-new-script').addEventListener('click', openNewScriptEditor);
    document.getElementById('library-grid').addEventListener('click', handleLibraryGridClick);

    updateLineNumbers();
}

function syncLineNumbersScroll() {
    const editor = document.getElementById('code-editor');
    document.getElementById('line-numbers').scrollTop = editor.scrollTop;
}

function handleEditorKeyDown(event) {
    if (event.key === 'Tab') {
        event.preventDefault();
        const editor = event.currentTarget;
        editor.setRangeText('    ', editor.selectionStart, editor.selectionEnd, 'end');
    }
}

function updateLineNumbers() {
    const editor = document.getElementById('code-editor');
    const lineNumbersElem = document.getElementById('line-numbers');
    const lineCount = editor.value.split('\n').length;

    const numbers = new Array(lineCount);
    for (let i = 0; i < lineCount; i++) {
        numbers[i] = i + 1;
    }
    lineNumbersElem.textContent = numbers.join('\n');
    lineNumbersElem.scrollTop = editor.scrollTop;
}

function insertSnippet(text) {
    const editor = document.getElementById('code-editor');
    const start = editor.selectionStart;
    const end = editor.selectionEnd;
    const val = editor.value;

    let insertText = text;
    if (text.endsWith('\n') && start > 0 && val[start - 1] !== '\n') {
        insertText = '\n' + text;
    }

    editor.setRangeText(insertText, start, end, 'end');
    editor.focus();
    updateLineNumbers();
}

function onScriptFormatChange() {
    const formatSelect = document.getElementById('script-format');
    const titleInput = document.getElementById('script-title');
    const name = titleInput.value.trim();

    if (formatSelect.value !== 'javascript' && /\.txt$/i.test(name)) {
        return;
    }
    const currentExt = formatSelect.value === 'javascript' ? '.js' : '.ducky';
    let baseName = name.replace(/\.(ducky|txt|js)$/i, '');
    if (!baseName) {
        baseName = 'untitled_payload';
    }
    titleInput.value = baseName + currentExt;
}

function readEditorTypingDelay() {
    const raw = Number.parseInt(document.getElementById('editor-typing-delay').value, 10);
    if (Number.isNaN(raw)) {
        return undefined;
    }
    return Math.max(0, raw);
}

async function executePayload(script, name, type) {
    if (!script.trim()) {
        appendLog('WARN', 'EDITOR', 'Cannot execute empty script.');
        return;
    }

    const runBtn = document.getElementById('btn-run-script');
    const stopBtn = document.getElementById('btn-stop-script');
    if (runBtn.disabled) return;

    runBtn.disabled = true;
    stopBtn.disabled = false;

    const requestBody = { script, name, type };
    const typingDelayMs = readEditorTypingDelay();
    if (typingDelayMs !== undefined) {
        requestBody.typingDelayMs = typingDelayMs;
    }

    appendLog('INFO', 'RUNNER', `Triggering execution for payload: ${name} (${type})...`);

    try {
        const response = await apiCall('/api/run', 'POST', requestBody);
        if (response.jobId && state.lastTerminalJobId !== response.jobId) {
            updateJobStatusUI({
                id: response.jobId,
                name,
                status: 'RUNNING',
                startedAt: new Date().toISOString()
            });
        }
    } catch (err) {
        appendLog('ERROR', 'RUNNER', `Failed to launch script: ${err.message}`);
        runBtn.disabled = false;
        stopBtn.disabled = true;
    }
}

async function runScript() {
    const code = document.getElementById('code-editor').value;
    const name = document.getElementById('script-title').value.trim() || 'inline_payload';
    const type = document.getElementById('script-format').value;
    await executePayload(code, name, type);
}

async function stopScript() {
    appendLog('WARN', 'RUNNER', 'Requesting emergency stop for active payload job...');

    try {
        await apiCall('/api/stop', 'POST', {});
        appendLog('INFO', 'RUNNER', 'Job stopped.');
        await syncJobStatus();
    } catch (err) {
        appendLog('ERROR', 'RUNNER', `Failed to stop job: ${err.message}`);
    }
}

async function syncJobStatus() {
    try {
        const job = await apiCall('/api/job');
        if (!job) {
            clearJobTimer();
            document.getElementById('btn-run-script').disabled = false;
            document.getElementById('btn-stop-script').disabled = true;
            document.getElementById('job-status-bar').classList.add('hidden');
            state.jobStartTime = null;
            return;
        }
        updateJobStatusUI(job);
    } catch (err) {
        console.warn('Failed to sync job status:', err);
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
    const timerDisp = document.getElementById('job-timer-display');
    const runBtn = document.getElementById('btn-run-script');
    const stopBtn = document.getElementById('btn-stop-script');

    const status = String(job.status).toUpperCase();
    const isTerminal = status === 'COMPLETED' || status === 'FAILED' || status === 'STOPPED';

    if (isTerminal) {
        if (job.id) {
            state.lastTerminalJobId = job.id;
        }
        clearJobTimer();
        runBtn.disabled = false;
        stopBtn.disabled = true;

        if (status === 'FAILED') {
            jobBar.classList.remove('hidden');
            pill.textContent = status;
            pill.className = 'job-status-pill failed';
            idDisp.textContent = `Job ID: ${job.id || '---'}`;
            nameDisp.textContent = job.error ? `Error: ${job.error}` : `Script: ${job.name || '---'}`;
            if (job.startedAt && job.finishedAt) {
                const dur = Math.max(0, (Date.parse(job.finishedAt) - Date.parse(job.startedAt)) / 1000).toFixed(1);
                timerDisp.textContent = `Duration: ${dur}s`;
            }
        } else {
            jobBar.classList.add('hidden');
        }
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

    if (job.startedAt) {
        const parsed = Date.parse(job.startedAt);
        if (!Number.isNaN(parsed)) {
            state.jobStartTime = parsed;
        }
    }
    if (!state.jobStartTime) {
        state.jobStartTime = Date.now();
    }

    const updateTimerText = () => {
        const elapsed = Math.max(0, (Date.now() - state.jobStartTime) / 1000).toFixed(1);
        timerDisp.textContent = `Duration: ${elapsed}s`;
    };
    updateTimerText();
    clearJobTimer();
    state.jobTimerInterval = setInterval(updateTimerText, 100);
}

// Live Console Output Terminal
function matchesLogFilter(level, source, filter) {
    if (filter === 'ALL') return true;
    if (filter === 'HID') return source === 'HID' || source === 'DUCKY' || source === 'JS';
    return level === filter;
}

function createLogEntry(level, source, message) {
    const normLevel = String(level).toUpperCase();
    const normSource = String(source).toUpperCase();
    const timeStr = new Date().toLocaleTimeString('en-US', { hour12: false });
    const tagVariant = normLevel === 'WARN' || normLevel === 'ERROR' ? normLevel.toLowerCase()
        : (normSource === 'HID' ? 'hid' : 'info');

    const entry = document.createElement('div');
    entry.className = `log-entry log-${normLevel.toLowerCase()}`;
    entry.dataset.level = normLevel;
    entry.dataset.source = normSource;

    const timeSpan = document.createElement('span');
    timeSpan.className = 'log-time';
    timeSpan.textContent = `[${timeStr}]`;

    const tagSpan = document.createElement('span');
    tagSpan.className = `log-tag tag-${tagVariant}`;
    tagSpan.textContent = normSource;

    const msgSpan = document.createElement('span');
    msgSpan.className = 'log-msg';
    msgSpan.textContent = String(message);

    entry.append(timeSpan, tagSpan, msgSpan);
    return entry;
}

function appendLog(level, source, message) {
    const terminal = document.getElementById('terminal-output');
    const currentFilter = document.getElementById('log-filter').value;
    const entry = createLogEntry(level, source, message);

    entry.style.display = matchesLogFilter(entry.dataset.level, entry.dataset.source, currentFilter) ? 'flex' : 'none';
    terminal.appendChild(entry);

    while (terminal.childElementCount > 500) {
        terminal.firstElementChild.remove();
    }
    terminal.scrollTop = terminal.scrollHeight;
}

function clearConsole() {
    const terminal = document.getElementById('terminal-output');
    terminal.replaceChildren(createLogEntry('INFO', 'SYS', 'Console cleared.'));
    filterTerminalLogs();
}

function filterTerminalLogs() {
    const filter = document.getElementById('log-filter').value;
    const entries = document.querySelectorAll('#terminal-output .log-entry');

    for (const entry of entries) {
        const visible = matchesLogFilter(entry.dataset.level, entry.dataset.source, filter);
        entry.style.display = visible ? 'flex' : 'none';
    }
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

function createActionButton(className, action, scriptName, labelText) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = className;
    btn.dataset.action = action;
    btn.dataset.name = scriptName;
    btn.textContent = labelText;
    return btn;
}

function renderLibrary() {
    const grid = document.getElementById('library-grid');
    const query = document.getElementById('library-search-input').value.toLowerCase().trim();
    const filtered = state.scripts.filter((s) =>
        s.name.toLowerCase().includes(query) ||
        s.description.toLowerCase().includes(query)
    );

    if (filtered.length === 0) {
        const emptyCard = document.createElement('div');
        emptyCard.className = 'glass-card library-empty';
        const msg = document.createElement('p');
        msg.textContent = 'No saved payloads found in library.';
        emptyCard.appendChild(msg);
        grid.replaceChildren(emptyCard);
        return;
    }

    const fragment = document.createDocumentFragment();
    for (const s of filtered) {
        const isJS = s.type === 'javascript';

        const card = document.createElement('div');
        card.className = 'script-card';

        const top = document.createElement('div');

        const header = document.createElement('div');
        header.className = 'script-card-header';

        const title = document.createElement('span');
        title.className = 'script-card-title';
        title.textContent = s.name;

        const badge = document.createElement('span');
        badge.className = `script-badge ${isJS ? 'badge-js' : 'badge-ducky'}`;
        badge.textContent = isJS ? 'JavaScript' : 'DuckyScript';

        header.append(title, badge);

        const desc = document.createElement('p');
        desc.className = 'script-card-desc';
        desc.textContent = s.description;

        const preview = document.createElement('div');
        preview.className = 'script-code-preview';
        preview.textContent = s.content;

        top.append(header, desc, preview);

        const actions = document.createElement('div');
        actions.className = 'script-card-actions';

        const loadBtn = createActionButton('btn btn-xs btn-primary', 'load', s.name, 'Load in Editor');
        const runBtn = createActionButton('btn btn-xs btn-success', 'run', s.name, 'Quick Run');
        const deleteBtn = createActionButton('btn btn-xs btn-danger', 'delete', s.name, 'Delete');
        deleteBtn.setAttribute('aria-label', `Delete ${s.name}`);

        actions.append(loadBtn, runBtn, deleteBtn);
        card.append(top, actions);
        fragment.appendChild(card);
    }

    grid.replaceChildren(fragment);
}

function handleLibraryGridClick(event) {
    const btn = event.target.closest('button[data-action]');
    if (!btn) return;

    const { action, name } = btn.dataset;
    if (action === 'load') {
        loadScriptToEditor(name);
    } else if (action === 'run') {
        quickRunScript(name);
    } else if (action === 'delete') {
        deleteScriptFromLibrary(name);
    }
}

function loadScriptToEditor(name) {
    const s = state.scripts.find((item) => item.name === name);
    if (!s) return;

    document.getElementById('script-title').value = s.name;
    document.getElementById('script-format').value = s.type;
    document.getElementById('code-editor').value = s.content;
    updateLineNumbers();
    switchTab('editor');
    appendLog('INFO', 'LIBRARY', `Loaded payload [${s.name}] into editor.`);
}

async function quickRunScript(name) {
    const s = state.scripts.find((item) => item.name === name);
    if (!s) return;

    switchTab('editor');
    await executePayload(s.content, s.name, s.type);
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

    try {
        const res = await apiCall('/api/scripts', 'POST', { name, type, content });
        if (res && res.name) {
            titleInput.value = res.name;
        }
        appendLog('INFO', 'LIBRARY', `Payload [${titleInput.value}] saved to library.`);
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

function openNewScriptEditor() {
    document.getElementById('script-title').value = 'new_payload.ducky';
    document.getElementById('script-format').value = 'duckyscript';
    document.getElementById('code-editor').value = 'REM New Raspiducky Payload\nDELAY 1000\n';
    updateLineNumbers();
    switchTab('editor');
}
