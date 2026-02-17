// Analytics + Admin moved to settings.html

// ──── Video Player ────
// Plex-style streaming:
// - Native formats (MP4/WebM): served directly with range requests (native seeking)
// - Non-native formats (MKV/AVI): remuxed to MPEG-TS on-the-fly via FFmpeg pipe
//   Video copied as-is, audio transcoded to AAC if needed. mpegts.js handles playback.
// - Seeking in MPEGTS streams: restart stream with ?start= parameter
let currentStreamInfo = null;
let currentPlayMode = null; // 'direct', 'mpegts', 'hls'
let knownDuration = 0; // Total duration from DB
let seekOffset = 0; // FFmpeg -ss offset for MPEGTS streams

// ── MTV Music Video Overlay State ──
let mtvOverlayData = null;    // {song_title, artist_name, album_title, record_label, year}
let mtvStartShown = false;    // Whether start overlay has been shown
let mtvEndShown = false;      // Whether end overlay has been triggered
let mtvOverlayTimer = null;   // Auto-hide timer

// ── Skip Segment State ──
let currentSegments = [];     // MediaSegment[] for current media
let activeSegment = null;     // Currently active segment (user is inside it)
let skipPrefs = null;         // UserSkipPreference
let skipPrefsLoaded = false;

// Load user skip preferences (cached for session)
async function loadSkipPrefs() {
    if (skipPrefsLoaded) return;
    const res = await api('GET', '/settings/skip');
    if (res.success) skipPrefs = res.data;
    else skipPrefs = { skip_intros: false, skip_credits: false, skip_recaps: false, show_skip_button: true };
    skipPrefsLoaded = true;
}

// Load segments for a media item
async function loadSegments(mediaId) {
    currentSegments = [];
    activeSegment = null;
    const res = await api('GET', '/media/' + mediaId + '/segments');
    if (res.success && res.data) currentSegments = res.data;
}

// Check segments against current playback time (called from updatePlayerUI)
function checkSegments() {
    if (!currentSegments.length || !skipPrefs) return;
    const video = document.getElementById('videoPlayer');
    const currentTime = video.currentTime + seekOffset;
    const btn = document.getElementById('skipSegmentBtn');
    const label = document.getElementById('skipSegmentLabel');

    let foundSegment = null;
    for (const seg of currentSegments) {
        // Show button 2 seconds before segment starts, hide after it ends
        if (currentTime >= seg.start_seconds - 2 && currentTime < seg.end_seconds) {
            foundSegment = seg;
            break;
        }
    }

    if (foundSegment && foundSegment !== activeSegment) {
        activeSegment = foundSegment;
        const typeLabel = { intro: 'Skip Intro', credits: 'Skip Credits', recap: 'Skip Recap', preview: 'Skip Preview' };
        label.textContent = typeLabel[foundSegment.segment_type] || 'Skip';

        // Auto-skip check
        const autoSkip = (foundSegment.segment_type === 'intro' && skipPrefs.skip_intros)
            || (foundSegment.segment_type === 'credits' && skipPrefs.skip_credits)
            || (foundSegment.segment_type === 'recap' && skipPrefs.skip_recaps);

        if (autoSkip && currentTime >= foundSegment.start_seconds) {
            // Auto-skip: jump to end of segment
            performSkip(foundSegment);
            return;
        }

        // Show skip button if user has show_skip_button enabled
        if (skipPrefs.show_skip_button) {
            btn.style.display = 'flex';
        }
    } else if (!foundSegment && activeSegment) {
        // Left the segment region
        activeSegment = null;
        btn.style.display = 'none';
    }
}

// Perform a skip to end of a segment
function performSkip(seg) {
    const video = document.getElementById('videoPlayer');
    const btn = document.getElementById('skipSegmentBtn');
    btn.style.display = 'none';
    activeSegment = null;

    if (currentPlayMode === 'mpegts') {
        startMpegtsPlay(currentMediaId, localStorage.getItem('token'), seg.end_seconds);
    } else {
        video.currentTime = seg.end_seconds - seekOffset;
    }
}

// Skip button click handler
function skipCurrentSegment() {
    if (activeSegment) performSkip(activeSegment);
}

// Destroy all active players
function destroyPlayers() {
    if (hlsPlayer) { hlsPlayer.destroy(); hlsPlayer = null; }
    if (mpegtsPlayer) {
        mpegtsPlayer.pause();
        mpegtsPlayer.unload();
        mpegtsPlayer.detachMediaElement();
        mpegtsPlayer.destroy();
        mpegtsPlayer = null;
    }
    if (dashPlayer) { dashPlayer.reset(); dashPlayer = null; }
}

async function playMedia(mediaId, title) {
    // Check if this item has multiple editions — show picker if so
    const edCheck = await api('GET', '/media/' + mediaId + '/editions');
    if (edCheck.success && edCheck.data.has_editions && edCheck.data.editions && edCheck.data.editions.length > 1) {
        showEditionPicker(edCheck.data.editions, title);
        return;
    }
    playMediaDirect(mediaId, title);
}

function watchTrailer(trailerUrl, title) {
    const overlay = document.getElementById('playerOverlay');
    const video = document.getElementById('videoPlayer');
    document.getElementById('playerTitle').textContent = 'Trailer: ' + title;
    overlay.classList.add('active');

    // Check if YouTube URL
    const ytMatch = trailerUrl.match(/(?:youtube\.com\/watch\?v=|youtu\.be\/)([a-zA-Z0-9_-]+)/);
    if (ytMatch) {
        // YouTube embed
        const container = video.parentElement;
        video.style.display = 'none';
        let iframe = document.getElementById('trailerIframe');
        if (!iframe) {
            iframe = document.createElement('iframe');
            iframe.id = 'trailerIframe';
            iframe.style.cssText = 'width:100%;height:100%;border:none;position:absolute;top:0;left:0;';
            iframe.allow = 'autoplay; encrypted-media';
            iframe.allowFullscreen = true;
            container.appendChild(iframe);
        }
        iframe.style.display = 'block';
        iframe.src = 'https://www.youtube-nocookie.com/embed/' + ytMatch[1] + '?autoplay=1&rel=0';
    } else {
        // Direct URL
        video.src = trailerUrl;
        video.play().catch(() => {});
    }
}

async function playMediaDirect(mediaId, title) {
    currentMediaId = mediaId;
    const overlay = document.getElementById('playerOverlay');
    const video = document.getElementById('videoPlayer');
    document.getElementById('playerTitle').textContent = title;
    overlay.classList.add('active');
    const token = localStorage.getItem('token');

    // Load skip segments, scene markers, and preferences in parallel with stream info
    loadSkipPrefs();
    loadSegments(mediaId);
    loadMarkers(mediaId);

    // Fetch stream info
    const info = await api('GET', `/stream/${mediaId}/info`);
    currentStreamInfo = info.success ? info.data : null;
    knownDuration = currentStreamInfo ? (currentStreamInfo.duration_seconds || 0) : 0;
    seekOffset = 0;

    const sel = document.getElementById('qualitySelect');
    let options = '';

    if (currentStreamInfo) {
        const nativeLabel = currentStreamInfo.native_resolution
            ? `Original (${currentStreamInfo.native_resolution}${currentStreamInfo.needs_remux ? ' \u00b7 direct stream' : ''})`
            : 'Original';
        options += `<option value="direct" selected>${nativeLabel}</option>`;

        if (currentStreamInfo.transcode_qualities) {
            currentStreamInfo.transcode_qualities.forEach(q => {
                options += `<option value="transcode:${q}">${q} (transcode)</option>`;
            });
        }
        // DASH option for Chrome/Android (P12-04)
        if (typeof dashjs !== 'undefined') {
            options += '<option value="dash">DASH (adaptive)</option>';
        }
    } else {
        options = '<option value="direct" selected>Original</option>';
    }
    sel.innerHTML = options;

    // Populate subtitle selector
    const subSel = document.getElementById('subtitleSelect');
    let subOpts = '<option value="off">Subtitles Off</option>';
    if (currentStreamInfo && currentStreamInfo.subtitles && currentStreamInfo.subtitles.length > 0) {
        currentStreamInfo.subtitles.forEach(sub => {
            const lang = sub.language || 'Unknown';
            const src = sub.source === 'embedded' ? 'emb' : 'ext';
            const flags = [sub.is_forced ? 'forced' : '', sub.is_sdh ? 'SDH' : ''].filter(Boolean).join(',');
            const label = sub.title || `${lang.toUpperCase()} (${src}${flags ? ' ' + flags : ''})`;
            subOpts += `<option value="${sub.id}">${label}</option>`;
        });
        subSel.style.display = '';
    } else {
        subSel.style.display = 'none';
    }
    subSel.innerHTML = subOpts;

    // Populate audio track selector
    const audioSel = document.getElementById('audioTrackSelect');
    let audioOpts = '';
    if (currentStreamInfo && currentStreamInfo.audio_tracks && currentStreamInfo.audio_tracks.length > 1) {
        currentStreamInfo.audio_tracks.forEach(track => {
            const lang = track.language || 'Unknown';
            const codec = track.codec ? track.codec.toUpperCase() : '';
            const ch = track.channels ? track.channels + 'ch' : '';
            const label = track.title || `${lang.toUpperCase()} ${codec} ${ch}`.trim();
            const sel = track.is_default ? ' selected' : '';
            audioOpts += `<option value="${track.stream_index}"${sel}>${label}</option>`;
        });
        audioSel.style.display = '';
    } else {
        audioSel.style.display = 'none';
    }
    audioSel.innerHTML = audioOpts;

    // Render chapter markers on seek bar
    renderChapterMarkers(currentStreamInfo);

    // Populate chapter selector dropdown
    const chapSel = document.getElementById('chapterSelect');
    if (chapSel && currentStreamInfo && currentStreamInfo.chapters && currentStreamInfo.chapters.length > 0) {
        let chapOpts = '<option value="">Chapters</option>';
        currentStreamInfo.chapters.forEach(ch => {
            chapOpts += `<option value="${ch.start_seconds}">${ch.title || 'Chapter'} (${formatTime(ch.start_seconds)})</option>`;
        });
        chapSel.innerHTML = chapOpts;
        chapSel.style.display = '';
    } else if (chapSel) {
        chapSel.style.display = 'none';
    }

    // Show the add marker button
    const markerBtn = document.getElementById('addMarkerBtn');
    if (markerBtn) markerBtn.style.display = '';

    // MTV music video overlay setup
    setupMtvOverlay(currentStreamInfo);

    // Start playback — MPEGTS for non-native formats, direct for native
    if (currentStreamInfo && currentStreamInfo.needs_remux) {
        startMpegtsPlay(mediaId, token, 0);
    } else {
        startDirectPlay(mediaId, token, 0);
    }
    video.addEventListener('timeupdate', updatePlayerUI);
    video.addEventListener('play', updatePlayPauseIcon);
    video.addEventListener('pause', updatePlayPauseIcon);
}

function playDirect(mediaId, title) {
    currentMediaId = mediaId;
    const overlay = document.getElementById('playerOverlay');
    const video = document.getElementById('videoPlayer');
    document.getElementById('playerTitle').textContent = title;
    overlay.classList.add('active');
    const token = localStorage.getItem('token');

    const sel = document.getElementById('qualitySelect');
    sel.innerHTML = '<option value="direct" selected>Original</option>';

    startDirectPlay(mediaId, token, 0);
    video.addEventListener('timeupdate', updatePlayerUI);
    video.addEventListener('play', updatePlayPauseIcon);
    video.addEventListener('pause', updatePlayPauseIcon);
}

// Direct play for native browser formats (MP4/WebM) — supports range requests & seeking
function startDirectPlay(mediaId, token, startSec) {
    const video = document.getElementById('videoPlayer');
    destroyPlayers();
    currentPlayMode = 'direct';
    seekOffset = 0;

    const url = `/api/v1/stream/${mediaId}/direct?token=${encodeURIComponent(token)}`;
    video.src = url;
    if (startSec > 0) {
        video.currentTime = startSec;
    }
    video.play().catch(e => {
        console.warn('Direct play starting...', e);
        setTimeout(() => video.play().catch(() => {}), 1000);
    });
}

// MPEGTS play for non-native formats (MKV/AVI) — Plex-style direct stream
function startMpegtsPlay(mediaId, token, startSec) {
    const video = document.getElementById('videoPlayer');
    destroyPlayers();
    currentPlayMode = 'mpegts';
    seekOffset = startSec || 0;

    let url = `/api/v1/stream/${mediaId}/direct?token=${encodeURIComponent(token)}`;
    if (seekOffset > 0) {
        url += `&start=${seekOffset.toFixed(1)}`;
    }
    // Pass selected audio track if set
    if (currentStreamInfo && currentStreamInfo.selectedAudioTrack !== undefined) {
        url += `&audio=${currentStreamInfo.selectedAudioTrack}`;
    }

    mpegtsPlayer = mpegts.createPlayer({
        type: 'mpegts',
        isLive: true, // piped stream (no defined end)
        url: url,
        duration: knownDuration ? knownDuration * 1000 : undefined,
    }, {
        enableStashBuffer: true,
        stashInitialSize: 512 * 1024, // 512KB initial buffer
        fixAudioTimestampGap: true, // auto-fix A/V sync gaps
        lazyLoad: false,
        autoCleanupSourceBuffer: true,
        autoCleanupMaxBackwardDuration: 300,
        autoCleanupMinBackwardDuration: 120,
    });

    mpegtsPlayer.attachMediaElement(video);
    mpegtsPlayer.load();
    mpegtsPlayer.play();

    mpegtsPlayer.on(mpegts.Events.ERROR, (errorType, errorDetail, errorInfo) => {
        console.error('mpegts.js error:', errorType, errorDetail, errorInfo);
        if (errorType === mpegts.ErrorTypes.NETWORK_ERROR) {
            toast('Stream interrupted — retrying...', 'info');
        } else {
            toast('Playback error: ' + errorDetail, 'error');
        }
    });
}

// HLS play for quality-specific transcodes
function startHLSPlay(mediaId, quality, token) {
    const video = document.getElementById('videoPlayer');
    destroyPlayers();
    currentPlayMode = 'hls';
    seekOffset = 0;

    const masterUrl = `/api/v1/stream/${mediaId}/master.m3u8?token=${encodeURIComponent(token)}`;
    hlsPlayer = new Hls({
        xhrSetup: (xhr, url) => {
            const sep = url.includes('?') ? '&' : '?';
            xhr.open('GET', url + sep + 'token=' + encodeURIComponent(token), true);
        }
    });
    hlsPlayer.loadSource(masterUrl);
    hlsPlayer.attachMedia(video);
    hlsPlayer.on(Hls.Events.MANIFEST_PARSED, () => {
        const levels = hlsPlayer.levels;
        for (let i = 0; i < levels.length; i++) {
            if (levels[i].height + 'p' === quality || levels[i].name === quality) {
                hlsPlayer.currentLevel = i;
                break;
            }
        }
        video.play().catch(() => {});
    });
    hlsPlayer.on(Hls.Events.ERROR, (event, data) => {
        if (data.fatal) {
            console.error('HLS error:', data);
            if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
                toast('Transcoding starting... retrying in 3s', 'info');
                setTimeout(() => hlsPlayer && hlsPlayer.startLoad(), 3000);
            } else {
                toast('Playback error: ' + data.details, 'error');
            }
        }
    });
}

function changeQuality(value) {
    const token = localStorage.getItem('token');
    if (value === 'direct') {
        if (currentStreamInfo && currentStreamInfo.needs_remux) {
            startMpegtsPlay(currentMediaId, token, 0);
        } else {
            startDirectPlay(currentMediaId, token, 0);
        }
    } else if (value === 'dash') {
        startDASHPlay(currentMediaId, token);
    } else if (value.startsWith('transcode:')) {
        const quality = value.replace('transcode:', '');
        startHLSPlay(currentMediaId, quality, token);
    }
}

// DASH playback (P12-04)
let dashPlayer = null;
function startDASHPlay(mediaId, token) {
    const video = document.getElementById('videoPlayer');
    destroyPlayers();
    if (typeof dashjs !== 'undefined') {
        dashPlayer = dashjs.MediaPlayer().create();
        dashPlayer.initialize(video, '/api/v1/stream/' + mediaId + '/manifest.mpd?token=' + encodeURIComponent(token), true);
        currentPlayMode = 'dash';
    } else {
        toast('DASH.js not loaded, falling back to HLS', 'warning');
        startHLSPlay(mediaId, '720p', token);
    }
}

// ──── Edition Picker ────
function showEditionPicker(editions, title) {
    const list = document.getElementById('editionPickerList');
    list.innerHTML = editions.map(e => {
        const dur = e.duration_seconds ? formatDuration(e.duration_seconds) : '';
        const res = e.resolution || '';
        const codec = e.codec || '';
        const audio = e.audio_codec || '';
        const metaParts = [dur, res, codec, audio].filter(Boolean).join(' \u00b7 ');
        const defBadge = e.is_default ? '<span class="ep-default">Default</span>' : '';
        return `<div class="edition-picker-card" onclick="pickEditionAndPlay('${e.media_item_id}','${(e.display_name || e.title).replace(/'/g,"\\'")}')">
            <span class="ep-type">${e.edition_type} ${defBadge}</span>
            <span class="ep-meta">${metaParts}</span>
            <button class="ep-play">&#9654; Play</button>
        </div>`;
    }).join('');
    document.getElementById('editionPickerOverlay').classList.add('active');
}

function pickEditionAndPlay(mediaId, title) {
    closeEditionPicker();
    playMediaDirect(mediaId, title);
}

function closeEditionPicker() {
    document.getElementById('editionPickerOverlay').classList.remove('active');
}

// Close edition picker on overlay click
document.getElementById('editionPickerOverlay').addEventListener('click', function(e) {
    if (e.target === this) closeEditionPicker();
});

function closePlayer() {
    const overlay = document.getElementById('playerOverlay');
    const video = document.getElementById('videoPlayer');
    overlay.classList.remove('active');
    video.removeEventListener('play', updatePlayPauseIcon);
    video.removeEventListener('pause', updatePlayPauseIcon);
    video.pause();
    video.src = '';
    video.style.display = '';
    // Reset play/pause icon to play
    const ppBtn = document.getElementById('playPauseBtn');
    if(ppBtn) ppBtn.innerHTML = '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>';
    // Reset mute icon to unmuted
    const mBtn = document.getElementById('muteBtn');
    if(mBtn) mBtn.innerHTML = '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/></svg>';
    destroyPlayers();

    // Clean up trailer iframe
    const iframe = document.getElementById('trailerIframe');
    if (iframe) { iframe.src = ''; iframe.style.display = 'none'; }

    // Save watch progress
    if (currentMediaId && video.currentTime > 0) {
        const progress = Math.floor(video.currentTime + seekOffset);
        api('POST', '/watch/'+currentMediaId+'/progress', {
            progress_seconds: progress,
            duration_seconds: knownDuration || Math.floor(video.duration || 0)
        });
    }
    currentMediaId = null;
    currentStreamInfo = null;
    currentPlayMode = null;
    knownDuration = 0;
    seekOffset = 0;
    currentSegments = [];
    activeSegment = null;
    document.getElementById('skipSegmentBtn').style.display = 'none';

    // Clean up MTV overlay
    hideMtvOverlay();
    mtvOverlayData = null;
    mtvStartShown = false;
    mtvEndShown = false;
}

function updatePlayerUI() {
    const video = document.getElementById('videoPlayer');
    const fill = document.getElementById('playerProgressFill');
    const timeEl = document.getElementById('playerTime');

    const totalDuration = (knownDuration > 0) ? knownDuration
        : (isFinite(video.duration) ? video.duration : 0);
    const currentTime = video.currentTime + seekOffset;

    if (totalDuration > 0) {
        fill.style.width = Math.min((currentTime / totalDuration * 100), 100) + '%';
        timeEl.textContent = formatTime(currentTime) + ' / ' + formatTime(totalDuration);
    }

    // Check for skip segments
    checkSegments();

    // MTV overlay: show at start and 10s before end
    checkMtvOverlay(currentTime, totalDuration);
}

function togglePlay() {
    const v=document.getElementById('videoPlayer');
    v.paused?v.play():v.pause();
    updatePlayPauseIcon();
}
function updatePlayPauseIcon() {
    const v=document.getElementById('videoPlayer');
    const btn=document.getElementById('playPauseBtn');
    if(!v||!btn) return;
    btn.innerHTML = v.paused
        ? '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>'
        : '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>';
}

function skipBack() {
    const video = document.getElementById('videoPlayer');
    if (currentPlayMode === 'mpegts') {
        // MPEGTS: restart stream from new position
        const target = Math.max(0, video.currentTime + seekOffset - 10);
        startMpegtsPlay(currentMediaId, localStorage.getItem('token'), target);
    } else {
        video.currentTime = Math.max(0, video.currentTime - 10);
    }
}

function skipForward() {
    const video = document.getElementById('videoPlayer');
    if (currentPlayMode === 'mpegts') {
        // MPEGTS: restart stream from new position
        const target = video.currentTime + seekOffset + 10;
        startMpegtsPlay(currentMediaId, localStorage.getItem('token'), target);
    } else {
        video.currentTime += 10;
    }
}

function toggleMute() {
    const v=document.getElementById('videoPlayer');
    v.muted=!v.muted;
    updateMuteIcon();
}
function updateMuteIcon() {
    const v=document.getElementById('videoPlayer');
    const btn=document.getElementById('muteBtn');
    if(!v||!btn) return;
    btn.innerHTML = v.muted
        ? '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z"/></svg>'
        : '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/></svg>';
}
function toggleFullscreen() { const o=document.getElementById('playerOverlay'); document.fullscreenElement?document.exitFullscreen():o.requestFullscreen(); }

// ── Subtitle Track Handling ──

let activeSubtitleTrack = null;

async function changeSubtitle(subtitleId) {
    const video = document.getElementById('videoPlayer');

    // Remove any existing text tracks
    while (video.textTracks.length > 0) {
        const track = video.textTracks[0];
        track.mode = 'disabled';
    }
    // Remove track elements
    video.querySelectorAll('track').forEach(t => t.remove());

    if (subtitleId === 'off') {
        activeSubtitleTrack = null;
        return;
    }

    activeSubtitleTrack = subtitleId;
    const token = localStorage.getItem('token');
    const trackUrl = `/api/v1/stream/${currentMediaId}/subtitles/${subtitleId}?token=${encodeURIComponent(token)}`;

    const trackEl = document.createElement('track');
    trackEl.kind = 'subtitles';
    trackEl.src = trackUrl;
    trackEl.default = true;
    trackEl.label = 'Subtitles';
    video.appendChild(trackEl);

    // Enable the track after it's added
    setTimeout(() => {
        if (video.textTracks.length > 0) {
            video.textTracks[0].mode = 'showing';
        }
    }, 100);
}

// ── Audio Track Selection ──

function changeAudioTrack(streamIndex) {
    if (currentStreamInfo) {
        currentStreamInfo.selectedAudioTrack = parseInt(streamIndex);
    }
    // Restart stream with new audio track
    const token = localStorage.getItem('token');
    if (currentPlayMode === 'mpegts') {
        const currentTime = document.getElementById('videoPlayer').currentTime + seekOffset;
        startMpegtsPlay(currentMediaId, token, currentTime);
    } else if (currentPlayMode === 'hls') {
        // HLS transcode: audio selection handled server-side on next segment request
        toast('Audio track will apply on next transcode start', 'info');
    }
}

// ── Chapter Markers ──

function renderChapterMarkers(streamInfo) {
    const container = document.getElementById('playerChapters');
    container.innerHTML = '';

    if (!streamInfo || !streamInfo.chapters || streamInfo.chapters.length === 0) {
        return;
    }

    const duration = streamInfo.duration_seconds || 0;
    if (duration <= 0) return;

    streamInfo.chapters.forEach(chapter => {
        const pct = (chapter.start_seconds / duration) * 100;
        const marker = document.createElement('div');
        marker.className = 'chapter-marker';
        marker.style.left = pct + '%';
        if (chapter.title) {
            marker.title = chapter.title;
            marker.setAttribute('data-tooltip', chapter.title);
        }
        marker.onclick = function(e) {
            e.stopPropagation();
            seekToTime(chapter.start_seconds);
        };
        container.appendChild(marker);
    });
}

function seekToTime(targetSeconds) {
    const video = document.getElementById('videoPlayer');
    if (currentPlayMode === 'mpegts') {
        startMpegtsPlay(currentMediaId, localStorage.getItem('token'), targetSeconds);
    } else {
        video.currentTime = targetSeconds;
    }
}

function seekPlayer(e) {
    const video = document.getElementById('videoPlayer');
    const bar = document.getElementById('playerProgress');
    const pct = e.offsetX / bar.offsetWidth;

    const totalDuration = (knownDuration > 0) ? knownDuration
        : (isFinite(video.duration) ? video.duration : 0);
    if (totalDuration <= 0) return;

    const targetTime = pct * totalDuration;

    if (currentPlayMode === 'mpegts') {
        // MPEGTS: restart FFmpeg from seek position (Plex-style)
        startMpegtsPlay(currentMediaId, localStorage.getItem('token'), targetTime);
    } else {
        // Native MP4 or HLS: standard seeking
        video.currentTime = targetTime;
    }
}

// Keyboard shortcuts for player
document.addEventListener('keydown', (e) => {
    if (!document.getElementById('playerOverlay').classList.contains('active')) return;
    switch(e.key) {
        case ' ': e.preventDefault(); togglePlay(); break;
        case 'ArrowLeft': skipBack(); break;
        case 'ArrowRight': skipForward(); break;
        case 'f': toggleFullscreen(); break;
        case 'm': toggleMute(); break;
        case 'Escape': closePlayer(); break;
    }
});

// ──────────────────── MTV Music Video Overlay ────────────────────

function setupMtvOverlay(streamInfo) {
    mtvOverlayData = null;
    mtvStartShown = false;
    mtvEndShown = false;
    if (mtvOverlayTimer) { clearTimeout(mtvOverlayTimer); mtvOverlayTimer = null; }
    hideMtvOverlay();

    if (!streamInfo || !streamInfo.music_video) return;

    const mv = streamInfo.music_video;
    if (!mv.artist_name && !mv.song_title) return;

    mtvOverlayData = mv;

    // Populate overlay elements
    const titleEl = document.getElementById('mtvSongTitle');
    const artistEl = document.getElementById('mtvArtist');
    const albumEl = document.getElementById('mtvAlbum');
    const labelYearEl = document.getElementById('mtvLabelYear');

    titleEl.textContent = mv.song_title || '';
    artistEl.textContent = mv.artist_name || '';
    albumEl.textContent = mv.album_title || '';

    const parts = [];
    if (mv.record_label) parts.push(mv.record_label);
    if (mv.year) parts.push(mv.year);
    labelYearEl.textContent = parts.join(' / ');

    // Hide lines that have no data
    albumEl.style.display = mv.album_title ? '' : 'none';
    labelYearEl.style.display = parts.length > 0 ? '' : 'none';
}

function showMtvOverlay(autoHideSec) {
    const el = document.getElementById('mtvOverlay');
    if (!el || !mtvOverlayData) return;
    el.classList.add('visible');

    if (mtvOverlayTimer) clearTimeout(mtvOverlayTimer);
    if (autoHideSec > 0) {
        mtvOverlayTimer = setTimeout(() => {
            el.classList.remove('visible');
            mtvOverlayTimer = null;
        }, autoHideSec * 1000);
    }
}

function hideMtvOverlay() {
    const el = document.getElementById('mtvOverlay');
    if (el) el.classList.remove('visible');
    if (mtvOverlayTimer) { clearTimeout(mtvOverlayTimer); mtvOverlayTimer = null; }
}

function checkMtvOverlay(currentTime, totalDuration) {
    if (!mtvOverlayData || totalDuration <= 0) return;

    // Start overlay: show for 10s at the beginning
    if (!mtvStartShown && currentTime >= 0.5 && currentTime < 11) {
        mtvStartShown = true;
        showMtvOverlay(10);
    }

    // End overlay: show 10s before the end
    if (!mtvEndShown && totalDuration > 20 && currentTime >= totalDuration - 10) {
        mtvEndShown = true;
        showMtvOverlay(0); // stays until player closes
    }
}

// ──────────────────── Profile Avatars ────────────────────
