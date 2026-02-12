// ──── Picture-in-Picture (P12-03) ────
function togglePiP() {
    const video = document.getElementById('videoPlayer');
    if (!video) return;
    if (document.pictureInPictureElement) {
        document.exitPictureInPicture().catch(() => {});
    } else if (document.pictureInPictureEnabled && video.readyState >= 1) {
        video.requestPictureInPicture().catch(() => {
            toast('PiP not supported', 'error');
        });
    }
}

// ──── Watch Together / SyncPlay (P12-01) ────
let currentSyncSession = null;

async function createSyncSession(mediaId) {
    const res = await api('POST', '/sync/create', { media_item_id: mediaId });
    if (!res.success) { toast(res.error || 'Failed to create session', 'error'); return; }
    currentSyncSession = res.data;
    toast('Watch Together session created! Code: ' + res.data.invite_code);
    showSyncPanel(res.data.session_id, true);
}

async function joinSyncSession() {
    const code = prompt('Enter invite code:');
    if (!code) return;
    const res = await api('POST', '/sync/join', { invite_code: code.trim() });
    if (!res.success) { toast(res.error || 'Failed to join session', 'error'); return; }
    currentSyncSession = res.data;
    toast('Joined Watch Together session!');
    playMediaDirect(res.data.media_item_id, 'Watch Together');
    showSyncPanel(res.data.session_id, false);
}

function showSyncPanel(sessionId, isHost) {
    // Create floating sync panel
    let panel = document.getElementById('syncPanel');
    if (!panel) {
        panel = document.createElement('div');
        panel.id = 'syncPanel';
        panel.className = 'sync-panel';
        document.body.appendChild(panel);
    }
    panel.innerHTML = `
        <div class="sync-panel-header">
            <span>Watch Together</span>
            <button onclick="endSyncSession('${sessionId}')" class="sync-close">&times;</button>
        </div>
        <div class="sync-code">Code: <strong>${currentSyncSession.invite_code || ''}</strong></div>
        <div id="syncChat" class="sync-chat"></div>
        <div class="sync-input-row">
            <input type="text" id="syncChatInput" placeholder="Chat..." onkeypress="if(event.key==='Enter')sendSyncChat('${sessionId}')">
            <button onclick="sendSyncChat('${sessionId}')">Send</button>
        </div>`;
    panel.style.display = 'flex';
}

async function sendSyncChat(sessionId) {
    const input = document.getElementById('syncChatInput');
    if (!input || !input.value.trim()) return;
    await api('POST', '/sync/' + sessionId + '/chat', { message: input.value.trim() });
    input.value = '';
}

async function endSyncSession(sessionId) {
    await api('DELETE', '/sync/' + sessionId);
    const panel = document.getElementById('syncPanel');
    if (panel) panel.style.display = 'none';
    currentSyncSession = null;
}

// Listen for sync WebSocket events
function handleSyncWSMessage(event, data) {
    if (event === 'sync:action' && currentSyncSession) {
        const video = document.getElementById('videoPlayer');
        if (!video) return;
        if (data.action === 'play') video.play();
        else if (data.action === 'pause') video.pause();
        else if (data.action === 'seek') video.currentTime = data.current_time;
    }
    if (event === 'sync:chat' && currentSyncSession) {
        const chatEl = document.getElementById('syncChat');
        if (chatEl) {
            const msg = document.createElement('div');
            msg.className = 'sync-chat-msg';
            msg.innerHTML = `<strong>${data.user}</strong>: ${data.message}`;
            chatEl.appendChild(msg);
            chatEl.scrollTop = chatEl.scrollHeight;
        }
    }
    if (event === 'sync:ended') {
        const panel = document.getElementById('syncPanel');
        if (panel) panel.style.display = 'none';
        currentSyncSession = null;
        toast('Watch Together session ended');
    }
}

// ──── Cinema Mode (P12-02) ────
async function playCinemaMode(mediaId, title) {
    const res = await api('GET', '/cinema/queue/' + mediaId);
    if (!res.success || !res.data || res.data.length === 0) {
        playMediaDirect(mediaId, title);
        return;
    }
    const queue = res.data;
    let idx = 0;
    const overlay = document.getElementById('playerOverlay');
    const video = document.getElementById('videoPlayer');

    function playNext() {
        if (idx >= queue.length) { closePlayer(); return; }
        const item = queue[idx];
        const label = item.type === 'feature' ? item.title : (item.type === 'pre_roll' ? 'Pre-Roll' : 'Trailer: ' + item.title);
        document.getElementById('playerTitle').textContent = label;

        if (item.type !== 'feature') {
            // Show skip button for non-features
            let skipBtn = document.getElementById('cinemaSkipBtn');
            if (!skipBtn) {
                skipBtn = document.createElement('button');
                skipBtn.id = 'cinemaSkipBtn';
                skipBtn.className = 'cinema-skip-btn';
                skipBtn.textContent = 'Skip ▶';
                skipBtn.onclick = () => { idx++; playNext(); };
                overlay.appendChild(skipBtn);
            }
            skipBtn.style.display = 'block';
        } else {
            const skipBtn = document.getElementById('cinemaSkipBtn');
            if (skipBtn) skipBtn.style.display = 'none';
        }

        if (item.type === 'feature') {
            playMediaDirect(item.id, item.title);
        } else {
            // Play pre-roll/trailer directly
            video.src = '/api/v1/stream/' + item.id + '/direct?token=' + encodeURIComponent(localStorage.getItem('token'));
            video.play();
        }
        idx++;
    }

    overlay.classList.add('active');
    video.onended = playNext;
    playNext();
}

