// ──── Music Player (P13-01 / Music Overhaul) ────
const musicPlayer = {
    queue: [],
    currentIndex: -1,
    audio: new Audio(),
    nextAudio: new Audio(),
    isPlaying: false,
    audioCtx: null,
    gainNode: null,
    sourceNode: null,
    nextSourceNode: null,
    shuffleMode: false,
    repeatMode: 'off', // 'off', 'all', 'one'
    _nextGainDB: null,
    _queueVisible: false,

    initAudioContext() {
        if (this.audioCtx) return;
        this.audioCtx = new (window.AudioContext || window.webkitAudioContext)();
        this.gainNode = this.audioCtx.createGain();
        this.gainNode.connect(this.audioCtx.destination);
        this.sourceNode = this.audioCtx.createMediaElementSource(this.audio);
        this.sourceNode.connect(this.gainNode);
    },

    setGainDB(db) {
        if (!this.gainNode) return;
        if (db === null || db === undefined || db === 0) {
            this.gainNode.gain.value = 1.0;
        } else {
            this.gainNode.gain.value = Math.pow(10, db / 20);
        }
    },

    enqueue(items) {
        this.queue = this.queue.concat(items);
        this.updateUI();
    },

    playNext() {
        if (this.repeatMode === 'one') {
            this.playTrack();
            return;
        }
        if (this.currentIndex < this.queue.length - 1) {
            this.currentIndex++;
            this.playTrack();
        } else if (this.repeatMode === 'all' && this.queue.length > 0) {
            this.currentIndex = 0;
            this.playTrack();
        }
    },

    playPrev() {
        if (this.audio.currentTime > 3) {
            this.audio.currentTime = 0;
            return;
        }
        if (this.currentIndex > 0) {
            this.currentIndex--;
            this.playTrack();
        }
    },

    async playTrack() {
        if (this.currentIndex < 0 || this.currentIndex >= this.queue.length) return;
        this.initAudioContext();
        if (this.audioCtx.state === 'suspended') this.audioCtx.resume();
        const track = this.queue[this.currentIndex];
        const token = localStorage.getItem('token');
        this.audio.src = '/api/v1/stream/' + track.id + '/direct?token=' + encodeURIComponent(token);

        try {
            const info = await api('GET', '/stream/' + track.id + '/info');
            if (info.success && info.data && info.data.loudness_gain_db !== undefined) {
                this.setGainDB(info.data.loudness_gain_db);
            } else {
                this.setGainDB(0);
            }
        } catch(e) { this.setGainDB(0); }

        this.audio.play();
        this.isPlaying = true;
        this.updateUI();
        this.preBufferNext();
        this.loadLyrics(track.id);
        api('POST', '/watch/' + track.id + '/progress', { progress_seconds: 0, duration_seconds: track.duration_seconds || 0 });
    },

    toggle() {
        if (this.isPlaying) { this.audio.pause(); this.isPlaying = false; }
        else {
            if (this.audioCtx && this.audioCtx.state === 'suspended') this.audioCtx.resume();
            this.audio.play(); this.isPlaying = true;
        }
        this.updateUI();
    },

    async preBufferNext() {
        this._nextGainDB = null;
        if (this.currentIndex < this.queue.length - 1) {
            const next = this.queue[this.currentIndex + 1];
            const token = localStorage.getItem('token');
            this.nextAudio.src = '/api/v1/stream/' + next.id + '/direct?token=' + encodeURIComponent(token);
            this.nextAudio.preload = 'auto';

            // Pre-fetch gain for gapless crossover so no async delay at transition
            try {
                const info = await api('GET', '/stream/' + next.id + '/info');
                if (info.success && info.data && info.data.loudness_gain_db !== undefined) {
                    this._nextGainDB = info.data.loudness_gain_db;
                } else {
                    this._nextGainDB = 0;
                }
            } catch(e) { this._nextGainDB = 0; }
        }
    },

    _syncedLyrics: null,

    async loadLyrics(mediaId) {
        this._syncedLyrics = null;
        const res = await api('GET', '/media/' + mediaId + '/lyrics');
        const lyricsEl = document.getElementById('musicLyrics');
        if (!lyricsEl) return;
        if (res.success && res.data && res.data.content) {
            if (res.data.type === 'synced') {
                const lines = _parseLRC(res.data.content);
                this._syncedLyrics = lines;
                let html = '<div class="lyrics-synced">';
                lines.forEach((l, i) => {
                    html += `<div class="lyrics-line" data-lyric-idx="${i}" data-time="${l.time}">${escapeHtml(l.text)}</div>`;
                });
                html += '</div>';
                lyricsEl.innerHTML = html;
            } else {
                lyricsEl.innerHTML = '<pre class="lyrics-text">' + res.data.content.replace(/</g, '&lt;') + '</pre>';
            }
            lyricsEl.style.display = 'block';
        } else {
            lyricsEl.style.display = 'none';
        }
    },

    seek(e) {
        const bar = e.currentTarget;
        const rect = bar.getBoundingClientRect();
        const pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
        if (this.audio.duration) {
            this.audio.currentTime = pct * this.audio.duration;
        }
    },

    setVolume(e) {
        const bar = e.currentTarget;
        const rect = bar.getBoundingClientRect();
        const pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
        this.audio.volume = pct;
        const fill = document.getElementById('mmpVolumeFill');
        if (fill) fill.style.width = (pct * 100) + '%';
    },

    toggleShuffle() {
        this.shuffleMode = !this.shuffleMode;
        const btn = document.getElementById('mmpShuffleBtn');
        if (btn) btn.classList.toggle('active', this.shuffleMode);
        if (this.shuffleMode && this.queue.length > 1) {
            const current = this.queue[this.currentIndex];
            const rest = this.queue.filter((_, i) => i !== this.currentIndex);
            for (let i = rest.length - 1; i > 0; i--) {
                const j = Math.floor(Math.random() * (i + 1));
                [rest[i], rest[j]] = [rest[j], rest[i]];
            }
            this.queue = [current, ...rest];
            this.currentIndex = 0;
            this.preBufferNext();
        }
        this.updateQueuePanel();
    },

    toggleRepeat() {
        const modes = ['off', 'all', 'one'];
        const idx = modes.indexOf(this.repeatMode);
        this.repeatMode = modes[(idx + 1) % modes.length];
        const btn = document.getElementById('mmpRepeatBtn');
        if (btn) {
            btn.classList.toggle('active', this.repeatMode !== 'off');
            btn.textContent = this.repeatMode === 'one' ? '🔂' : '🔁';
        }
    },

    toggleQueue() {
        this._queueVisible = !this._queueVisible;
        const panel = document.getElementById('mmpQueuePanel');
        if (panel) panel.style.display = this._queueVisible ? 'block' : 'none';
        if (this._queueVisible) this.updateQueuePanel();
    },

    updateQueuePanel() {
        const panel = document.getElementById('mmpQueuePanel');
        if (!panel || !this._queueVisible) return;
        if (this.queue.length === 0) {
            panel.innerHTML = '<div class="mmp-queue-empty">Queue is empty</div>';
            return;
        }
        let html = '<div class="mmp-queue-header">Queue (' + this.queue.length + ' tracks)</div><div class="mmp-queue-list">';
        this.queue.forEach((t, i) => {
            const active = i === this.currentIndex ? ' mmp-queue-active' : '';
            html += `<div class="mmp-queue-item${active}" onclick="musicPlayer.currentIndex=${i};musicPlayer.playTrack()">`;
            html += `<span class="mmp-queue-num">${i + 1}</span>`;
            html += `<span class="mmp-queue-title">${escapeHtml(t.title || 'Unknown')}</span>`;
            html += `<span class="mmp-queue-artist">${escapeHtml(t.artist || '')}</span>`;
            html += `<button class="btn-icon mmp-queue-remove" onclick="event.stopPropagation();musicPlayer.removeFromQueue(${i})" title="Remove">&times;</button>`;
            html += '</div>';
        });
        html += '</div>';
        panel.innerHTML = html;
    },

    removeFromQueue(idx) {
        if (idx < 0 || idx >= this.queue.length) return;
        if (idx === this.currentIndex) return;
        this.queue.splice(idx, 1);
        if (idx < this.currentIndex) this.currentIndex--;
        this.updateQueuePanel();
        this.preBufferNext();
    },

    updateUI() {
        let bar = document.getElementById('musicMiniPlayer');
        if (!bar && this.queue.length > 0) {
            bar = document.createElement('div');
            bar.id = 'musicMiniPlayer';
            bar.className = 'music-mini-player';
            bar.innerHTML =
                '<div class="mmp-info"><span id="mmpTitle">\u2014</span><span id="mmpArtist"></span></div>' +
                '<div class="mmp-controls">' +
                    '<button id="mmpShuffleBtn" class="btn-icon mmp-ctrl-btn" onclick="musicPlayer.toggleShuffle()" title="Shuffle">&#128256;</button>' +
                    '<button class="btn-icon mmp-ctrl-btn" onclick="musicPlayer.playPrev()" title="Previous">&#9198;</button>' +
                    '<button id="mmpPlayBtn" class="btn-icon mmp-ctrl-btn mmp-play-btn" onclick="musicPlayer.toggle()">&#9654;</button>' +
                    '<button class="btn-icon mmp-ctrl-btn" onclick="musicPlayer.playNext()" title="Next">&#9197;</button>' +
                    '<button id="mmpRepeatBtn" class="btn-icon mmp-ctrl-btn" onclick="musicPlayer.toggleRepeat()" title="Repeat">&#128257;</button>' +
                '</div>' +
                '<div class="mmp-time"><span id="mmpTimeElapsed">0:00</span></div>' +
                '<div class="mmp-progress" onclick="musicPlayer.seek(event)"><div id="mmpProgressFill" class="mmp-progress-fill"></div></div>' +
                '<div class="mmp-time"><span id="mmpTimeTotal">0:00</span></div>' +
                '<div class="mmp-volume-wrap">' +
                    '<span class="mmp-vol-icon">&#128266;</span>' +
                    '<div class="mmp-volume" onclick="musicPlayer.setVolume(event)"><div id="mmpVolumeFill" class="mmp-volume-fill" style="width:100%"></div></div>' +
                '</div>' +
                '<button class="btn-icon mmp-ctrl-btn" onclick="musicPlayer.toggleQueue()" title="Queue">&#9776;</button>' +
                '<div id="musicLyrics" class="music-lyrics" style="display:none;"></div>' +
                '<div id="mmpQueuePanel" class="mmp-queue-panel" style="display:none;"></div>';
            document.body.appendChild(bar);
        }
        if (!bar) return;
        bar.style.display = this.queue.length > 0 ? 'flex' : 'none';
        const track = this.queue[this.currentIndex];
        if (track) {
            const titleEl = document.getElementById('mmpTitle');
            const artistEl = document.getElementById('mmpArtist');
            if (titleEl) titleEl.textContent = track.title || 'Unknown';
            if (artistEl) artistEl.textContent = track.artist || '';
        }
        const btn = document.getElementById('mmpPlayBtn');
        if (btn) btn.innerHTML = this.isPlaying ? '&#10074;&#10074;' : '&#9654;';
        this.updateQueuePanel();
    }
};

// Gapless crossover using pre-fetched gain (no async delay)
musicPlayer.audio.addEventListener('ended', () => {
    if (musicPlayer.repeatMode === 'one') {
        musicPlayer.audio.currentTime = 0;
        musicPlayer.audio.play();
        return;
    }
    if (musicPlayer.currentIndex < musicPlayer.queue.length - 1) {
        const oldAudio = musicPlayer.audio;
        musicPlayer.audio = musicPlayer.nextAudio;
        musicPlayer.nextAudio = oldAudio;
        musicPlayer.currentIndex++;

        if (musicPlayer.audioCtx && musicPlayer.gainNode) {
            if (musicPlayer.sourceNode) {
                try { musicPlayer.sourceNode.disconnect(); } catch(e) {}
            }
            musicPlayer.sourceNode = musicPlayer.audioCtx.createMediaElementSource(musicPlayer.audio);
            musicPlayer.sourceNode.connect(musicPlayer.gainNode);
        }

        // Apply pre-fetched gain immediately (no async call)
        musicPlayer.setGainDB(musicPlayer._nextGainDB || 0);

        musicPlayer.audio.play();
        musicPlayer.isPlaying = true;
        musicPlayer.preBufferNext();
        musicPlayer.updateUI();
        const track = musicPlayer.queue[musicPlayer.currentIndex];
        if (track) musicPlayer.loadLyrics(track.id);
    } else if (musicPlayer.repeatMode === 'all' && musicPlayer.queue.length > 0) {
        musicPlayer.currentIndex = 0;
        musicPlayer.playTrack();
    } else {
        musicPlayer.isPlaying = false;
        musicPlayer.updateUI();
    }
});

// Progress bar + time display update
musicPlayer.audio.addEventListener('timeupdate', () => {
    const fill = document.getElementById('mmpProgressFill');
    const elapsed = document.getElementById('mmpTimeElapsed');
    const total = document.getElementById('mmpTimeTotal');
    if (fill && musicPlayer.audio.duration) {
        fill.style.width = (musicPlayer.audio.currentTime / musicPlayer.audio.duration * 100) + '%';
    }
    if (elapsed) {
        elapsed.textContent = _fmtTime(musicPlayer.audio.currentTime);
    }
    if (total && musicPlayer.audio.duration) {
        total.textContent = _fmtTime(musicPlayer.audio.duration);
    }
});

function _fmtTime(sec) {
    if (!sec || isNaN(sec)) return '0:00';
    const m = Math.floor(sec / 60);
    const s = Math.floor(sec % 60);
    return m + ':' + String(s).padStart(2, '0');
}

// Parse LRC format: [mm:ss.xx] text or [mm:ss] text
function _parseLRC(content) {
    const lines = [];
    const re = /\[(\d{1,2}):(\d{2})(?:[.:]\d{1,3})?\]\s*(.*)/;
    for (const raw of content.split('\n')) {
        const m = re.exec(raw.trim());
        if (m) {
            const time = parseInt(m[1]) * 60 + parseInt(m[2]);
            const text = m[3].trim();
            if (text) lines.push({ time, text });
        }
    }
    lines.sort((a, b) => a.time - b.time);
    return lines;
}

// Highlight active synced lyrics line
musicPlayer.audio.addEventListener('timeupdate', () => {
    if (!musicPlayer._syncedLyrics || musicPlayer._syncedLyrics.length === 0) return;
    const ct = musicPlayer.audio.currentTime;
    let activeIdx = -1;
    for (let i = musicPlayer._syncedLyrics.length - 1; i >= 0; i--) {
        if (ct >= musicPlayer._syncedLyrics[i].time) {
            activeIdx = i;
            break;
        }
    }
    const lyricsEl = document.getElementById('musicLyrics');
    if (!lyricsEl) return;
    const allLines = lyricsEl.querySelectorAll('.lyrics-line');
    allLines.forEach((el, i) => {
        const isActive = i === activeIdx;
        el.classList.toggle('lyrics-line-active', isActive);
        if (isActive) el.scrollIntoView({ block: 'center', behavior: 'smooth' });
    });
});
