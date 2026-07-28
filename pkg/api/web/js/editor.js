/**
 * Script Execution Engine, Code Editor, Console Terminal & Payload Library Manager
 */

// Line Numbers Sync
function updateLineNumbers() {
    const editor = document.getElementById('code-editor');
    const lineNumbersElem = document.getElementById('line-numbers');
    if (!editor || !lineNumbersElem) return;

    const lines = editor.value.split('\n').length;
    let numbersHtml = '';
    for (let i = 1; i <= lines; i++) {
        numbersHtml += `${i}<br>`;
    }
    lineNumbersElem.innerHTML = numbersHtml;
}

function insertSnippet(text) {
    const editor = document.getElementById('code-editor');
    if (!editor) return;

    const start = editor.selectionStart;
    const end = editor.selectionEnd;
    const val = editor.value;

    editor.value = val.substring(0, start) + text + val.substring(end);
    editor.selectionStart = editor.selectionEnd = start + text.length;
    editor.focus();
    updateLineNumbers();
}

function onScriptFormatChange() {
    const formatSelect = document.getElementById('script-format');
    const titleInput = document.getElementById('script-title');
    if (!formatSelect || !titleInput) return;

    const currentExt = formatSelect.value === 'javascript' ? '.js' : '.ducky';
    let baseName = titleInput.value.replace(/\.(ducky|js)$/i, '');
    if (!baseName) baseName = 'untitled_payload';
    titleInput.value = baseName + currentExt;
}

// Script Execution Engine Controls
async function runScript() {
    const code = document.getElementById('code-editor').value;
    const name = document.getElementById('script-title').value.trim() || 'untitled_payload';
    const type = document.getElementById('script-format').value;

    if (!code.trim()) {
        appendLog('WARN', 'EDITOR', 'Cannot execute empty script.');
        return;
    }

    const runBtn = document.getElementById('btn-run-script');
    const stopBtn = document.getElementById('btn-stop-script');
    
    if (runBtn) runBtn.disabled = true;
    if (stopBtn) stopBtn.disabled = false;

    appendLog('INFO', 'RUNNER', `Triggering execution for payload: ${name} (${type})...`);

    try {
        const response = await apiCall('/api/run', 'POST', {
            script: code,
            name: name,
            type: type
        });

        if (response && response.jobId) {
            updateJobStatusUI({
                id: response.jobId,
                name: name,
                status: 'RUNNING',
                startedAt: new Date().toISOString()
            });
        }
    } catch (err) {
        appendLog('ERROR', 'RUNNER', `Failed to launch script: ${err.message}`);
        if (runBtn) runBtn.disabled = false;
        if (stopBtn) stopBtn.disabled = true;
    }
}

async function stopScript() {
    const runBtn = document.getElementById('btn-run-script');
    const stopBtn = document.getElementById('btn-stop-script');

    appendLog('WARN', 'RUNNER', 'Requesting emergency stop for active payload job...');

    try {
        await apiCall('/api/stop', 'POST', {});
        appendLog('INFO', 'RUNNER', 'Job stop signal dispatched successfully.');
    } catch (err) {
        appendLog('ERROR', 'RUNNER', `Failed to stop job: ${err.message}`);
    } finally {
        if (runBtn) runBtn.disabled = false;
        if (stopBtn) stopBtn.disabled = true;
    }
}

function updateJobStatusUI(job) {
    const jobBar = document.getElementById('job-status-bar');
    const pill = document.getElementById('job-status-pill');
    const idDisp = document.getElementById('job-id-display');
    const nameDisp = document.getElementById('job-name-display');
    const runBtn = document.getElementById('btn-run-script');
    const stopBtn = document.getElementById('btn-stop-script');

    if (!job || job.status === 'IDLE' || job.status === 'COMPLETED' || job.status === 'STOPPED' || job.status === 'FAILED') {
        if (jobBar) jobBar.classList.add('hidden');
        if (runBtn) runBtn.disabled = false;
        if (stopBtn) stopBtn.disabled = true;
        if (state.jobTimerInterval) {
            clearInterval(state.jobTimerInterval);
            state.jobTimerInterval = null;
        }
        return;
    }

    if (jobBar) jobBar.classList.remove('hidden');
    if (pill) {
        pill.textContent = job.status.toUpperCase();
        pill.className = `job-status-pill ${job.status.toLowerCase()}`;
    }
    if (idDisp) idDisp.textContent = `Job ID: ${job.id}`;
    if (nameDisp) nameDisp.textContent = `Script: ${job.name || job.scriptName || 'Active Payload'}`;

    if (runBtn) runBtn.disabled = true;
    if (stopBtn) stopBtn.disabled = false;

    // Start timer counter
    state.jobStartTime = Date.now();
    if (!state.jobTimerInterval) {
        state.jobTimerInterval = setInterval(() => {
            const elapsed = ((Date.now() - state.jobStartTime) / 1000).toFixed(1);
            const timerElem = document.getElementById('job-timer-display');
            if (timerElem) timerElem.textContent = `Duration: ${elapsed}s`;
        }, 100);
    }
}

// Live Console Output Terminal
function appendLog(level, source, message) {
    const terminal = document.getElementById('terminal-output');
    if (!terminal) return;

    const timeStr = new Date().toLocaleTimeString('en-US', { hour12: false });
    const tagClass = `tag-${level.toLowerCase()}`;
    
    const entry = document.createElement('div');
    entry.className = `log-entry log-${level.toLowerCase()}`;
    entry.setAttribute('data-level', level.toUpperCase());
    entry.innerHTML = `
        <span class="log-time">[${timeStr}]</span>
        <span class="log-tag ${tagClass}">${source}</span>
        <span class="log-msg">${escapeHtml(message)}</span>
    `;

    terminal.appendChild(entry);
    terminal.scrollTop = terminal.scrollHeight;
}

function clearConsole() {
    const terminal = document.getElementById('terminal-output');
    if (terminal) {
        terminal.innerHTML = `
            <div class="log-entry log-system">
                <span class="log-time">[${new Date().toLocaleTimeString('en-US', { hour12: false })}]</span>
                <span class="log-tag tag-info">SYS</span>
                <span class="log-msg">Console cleared.</span>
            </div>
        `;
    }
}

function filterTerminalLogs() {
    const filter = document.getElementById('log-filter').value;
    const entries = document.querySelectorAll('.terminal-body .log-entry');

    entries.forEach(entry => {
        const level = entry.getAttribute('data-level');
        if (filter === 'ALL' || filter === level) {
            entry.style.display = 'flex';
        } else {
            entry.style.display = 'none';
        }
    });
}

function escapeHtml(str) {
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
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
        console.warn('Could not fetch script library:', err);
        // Default built-in payload templates for initial display
        state.scripts = [
            {
                name: 'windows_reverse_shell.ducky',
                type: 'duckyscript',
                description: 'Windows CMD Powershell reverse shell payload template',
                content: 'REM Windows Powershell Launcher\nDELAY 1000\nGUI r\nDELAY 500\nSTRING powershell -NoP -NonI -W Hidden -Exec Bypass -Command "Write-Host Raspiducky Payload Executed!"\nENTER\n',
                updatedAt: new Date().toISOString()
            },
            {
                name: 'macos_terminal_opener.ducky',
                type: 'duckyscript',
                description: 'macOS Spotlight Terminal launcher & auto command execution',
                content: 'REM macOS Spotlight Launcher\nDELAY 1000\nGUI SPACE\nDELAY 500\nSTRING Terminal\nENTER\nDELAY 1000\nSTRING echo "Raspiducky macOS Payload Active!"\nENTER\n',
                updatedAt: new Date().toISOString()
            },
            {
                name: 'mouse_jiggler.js',
                type: 'javascript',
                description: 'JS HID script that moves cursor in circles to prevent system sleep',
                content: '// Raspiducky JS Mouse Jiggler\nconsole.log("Starting Mouse Jiggler loop...");\nfor (let i = 0; i < 10; i++) {\n    HID.moveMouse(10, 0);\n    HID.delay(200);\n    HID.moveMouse(0, 10);\n    HID.delay(200);\n    HID.moveMouse(-10, 0);\n    HID.delay(200);\n    HID.moveMouse(0, -10);\n    HID.delay(200);\n}\nconsole.log("Jiggler finished!");\n',
                updatedAt: new Date().toISOString()
            }
        ];
        renderLibrary();
    }
}

function renderLibrary() {
    const grid = document.getElementById('library-grid');
    const searchInput = document.getElementById('library-search-input');
    if (!grid) return;

    const query = searchInput ? searchInput.value.toLowerCase().trim() : '';
    const filtered = state.scripts.filter(s => 
        s.name.toLowerCase().includes(query) || 
        (s.description && s.description.toLowerCase().includes(query))
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
        return `
            <div class="script-card">
                <div>
                    <div class="script-card-header">
                        <span class="script-card-title">${escapeHtml(s.name)}</span>
                        <span class="script-badge ${badgeClass}">${typeLabel}</span>
                    </div>
                    <p class="script-card-desc">${escapeHtml(s.description || 'Payload script template')}</p>
                    <div class="script-code-preview">${escapeHtml(s.content || '')}</div>
                </div>
                <div class="script-card-actions">
                    <button class="btn btn-xs btn-primary" onclick="loadScriptToEditor('${escapeHtml(s.name)}')">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                        Load in Editor
                    </button>
                    <button class="btn btn-xs btn-success" onclick="quickRunScript('${escapeHtml(s.name)}')">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                        Quick Run
                    </button>
                    <button class="btn btn-xs btn-danger" onclick="deleteScriptFromLibrary('${escapeHtml(s.name)}')">
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
    document.getElementById('script-format').value = s.type || (s.name.endsWith('.js') ? 'javascript' : 'duckyscript');
    document.getElementById('code-editor').value = s.content || '';
    updateLineNumbers();
    switchTab('editor');
    appendLog('INFO', 'LIBRARY', `Loaded payload [${s.name}] into editor.`);
}

async function quickRunScript(name) {
    const s = state.scripts.find(item => item.name === name);
    if (!s) return;

    loadScriptToEditor(name);
    await runScript();
}

async function saveCurrentScript() {
    const name = document.getElementById('script-title').value.trim();
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
        await apiCall('/api/scripts', 'POST', payload);
        appendLog('INFO', 'LIBRARY', `Payload [${name}] saved to library successfully!`);
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
