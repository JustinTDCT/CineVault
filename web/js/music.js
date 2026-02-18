// ──── Music Player (P13-01) ────
const musicPlayer = {
    queue: [],
    currentIndex: -1,
    audio: new Audio(),
    nextAudio: new Audio(),
    isPlaying: false,
    audioCtx: null,
    gainNode: null,
    sourceNode: null,

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

    enqueue(items) { this.queue = this.queue.concat(items); this.updateUI(); },
    playNext() { if (this.currentIndex < this.queue.length - 1) { this.currentIndex++; this.playTrack(); } },
    playPrev() { if (this.currentIndex > 0) { this.currentIndex--; this.playTrack(); } },

    async playTrack() {
        if (this.currentIndex < 0 || this.currentIndex >= this.queue.length) return;
        this.initAudioContext();
        if (this.audioCtx.state === 'suspended') this.audioCtx.resume();
        const track = this.queue[this.currentIndex];
        const token = localStorage.getItem('token');
        this.audio.src = '/api/v1/stream/' + track.id + '/direct?token=' + encodeURIComponent(token);

        // Fetch stream info for loudness normalization gain
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

    preBufferNext() {
        if (this.currentIndex < this.queue.length - 1) {
            const next = this.queue[this.currentIndex + 1];
            const token = localStorage.getItem('token');
            this.nextAudio.src = '/api/v1/stream/' + next.id + '/direct?token=' + encodeURIComponent(token);
            this.nextAudio.preload = 'auto';
        }
    },

    async loadLyrics(mediaId) {
        const res = await api('GET', '/media/' + mediaId + '/lyrics');
        const lyricsEl = document.getElementById('musicLyrics');
        if (!lyricsEl) return;
        if (res.success && res.data && res.data.content) {
            lyricsEl.innerHTML = '<pre class="lyrics-text">' + res.data.content.replace(/</g, '&lt;') + '</pre>';
            lyricsEl.style.display = 'block';
        } else {
            lyricsEl.style.display = 'none';
        }
    },

    updateUI() {
        let bar = document.getElementById('musicMiniPlayer');
        if (!bar && this.queue.length > 0) {
            bar = document.createElement('div');
            bar.id = 'musicMiniPlayer';
            bar.className = 'music-mini-player';
            bar.innerHTML = '<div class="mmp-info"><span id="mmpTitle">\u2014</span><span id="mmpArtist"></span></div>' +
                '<div class="mmp-controls">' +
                '<button onclick="musicPlayer.playPrev()">&#9198;</button>' +
                '<button id="mmpPlayBtn" onclick="musicPlayer.toggle()">&#9654;</button>' +
                '<button onclick="musicPlayer.playNext()">&#9197;</button>' +
                '</div>' +
                '<div class="mmp-progress"><div id="mmpProgressFill" class="mmp-progress-fill"></div></div>' +
                '<div id="musicLyrics" class="music-lyrics" style="display:none;"></div>';
            document.body.appendChild(bar);
        }
        if (!bar) return;
        bar.style.display = this.queue.length > 0 ? 'flex' : 'none';
        const track = this.queue[this.currentIndex];
        if (track) {
            document.getElementById('mmpTitle').textContent = track.title || 'Unknown';
            document.getElementById('mmpArtist').textContent = track.artist || '';
        }
        const btn = document.getElementById('mmpPlayBtn');
        if (btn) btn.innerHTML = this.isPlaying ? '&#10074;&#10074;' : '&#9654;';
    }
};

// Gapless crossover
musicPlayer.audio.addEventListener('ended', async () => {
    if (musicPlayer.currentIndex < musicPlayer.queue.length - 1) {
        const oldAudio = musicPlayer.audio;
        musicPlayer.audio = musicPlayer.nextAudio;
        musicPlayer.nextAudio = oldAudio;
        musicPlayer.currentIndex++;

        // Reconnect new audio element through the gain node
        if (musicPlayer.audioCtx && musicPlayer.gainNode) {
            if (musicPlayer.sourceNode) {
                try { musicPlayer.sourceNode.disconnect(); } catch(e) {}
            }
            musicPlayer.sourceNode = musicPlayer.audioCtx.createMediaElementSource(musicPlayer.audio);
            musicPlayer.sourceNode.connect(musicPlayer.gainNode);
        }

        // Fetch gain for the new track
        const track = musicPlayer.queue[musicPlayer.currentIndex];
        try {
            const info = await api('GET', '/stream/' + track.id + '/info');
            if (info.success && info.data && info.data.loudness_gain_db !== undefined) {
                musicPlayer.setGainDB(info.data.loudness_gain_db);
            } else {
                musicPlayer.setGainDB(0);
            }
        } catch(e) { musicPlayer.setGainDB(0); }

        musicPlayer.audio.play();
        musicPlayer.isPlaying = true;
        musicPlayer.preBufferNext();
        musicPlayer.updateUI();
        if (track) musicPlayer.loadLyrics(track.id);
    } else {
        musicPlayer.isPlaying = false;
        musicPlayer.updateUI();
    }
});

// Progress bar update
musicPlayer.audio.addEventListener('timeupdate', () => {
    const fill = document.getElementById('mmpProgressFill');
    if (fill && musicPlayer.audio.duration) {
        fill.style.width = (musicPlayer.audio.currentTime / musicPlayer.audio.duration * 100) + '%';
    }
});
