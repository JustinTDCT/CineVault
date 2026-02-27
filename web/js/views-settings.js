// Profile, stats, hub, and settings views
async function loadProfileView() {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Edit Profile</h2></div><div class="settings-grid" id="profileGrid"><div class="spinner"></div></div>`;

    const profileData = await api('GET', '/profile');
    if (!profileData.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Failed to load profile</div></div>'; return; }
    const u = profileData.data;

    // Get PIN length setting for validation
    let pinLength = 4;
    try {
        const flSettings = await api('GET', '/auth/fast-login/settings');
        if (flSettings.success && flSettings.data) pinLength = parseInt(flSettings.data.fast_login_pin_length) || 4;
    } catch(e) {}

    document.getElementById('profileGrid').innerHTML = `
        <div class="settings-card">
            <h3>Personal Information</h3>
            <div class="form-group"><label>Username</label><input type="text" value="${u.username || ''}" disabled style="opacity:0.6;cursor:not-allowed;"></div>
            <div class="edit-field-row">
                <div class="form-group"><label>First Name</label><input type="text" id="profFirstName" value="${u.first_name || ''}" placeholder="Enter first name"></div>
                <div class="form-group"><label>Last Name</label><input type="text" id="profLastName" value="${u.last_name || ''}" placeholder="Enter last name"></div>
            </div>
            <div class="form-group"><label>Email</label><input type="email" id="profEmail" value="${u.email || ''}" placeholder="Enter email"></div>
            <button class="btn-primary" onclick="saveProfile()">Save Changes</button>
        </div>
        <div class="settings-card">
            <h3>Security</h3>
            <div class="form-group">
                <label>New Password</label>
                <input type="password" id="profPassword" placeholder="Leave blank to keep current">
            </div>
            <div class="form-group">
                <label>Confirm Password</label>
                <input type="password" id="profPasswordConfirm" placeholder="Confirm new password">
            </div>
            <button class="btn-primary" onclick="saveProfilePassword()" style="margin-bottom:20px;">Change Password</button>
            <div style="border-top:1px solid rgba(0,217,255,0.1);padding-top:16px;margin-top:4px;">
                <h3 style="margin-bottom:12px;">Login PIN</h3>
                <p style="color:#8a9bae;font-size:0.82rem;margin-bottom:12px;">Set a ${pinLength}-digit PIN for quick login. ${u.has_pin ? '<span style="color:#51cf66;">PIN is set.</span>' : '<span style="color:#5a6a7f;">No PIN set.</span>'}</p>
                <div class="form-group">
                    <label>New PIN (${pinLength} digits)</label>
                    <input type="password" id="profPin" placeholder="Enter ${pinLength}-digit PIN" maxlength="${pinLength}" pattern="[0-9]*" inputmode="numeric">
                </div>
                <button class="btn-primary" onclick="saveProfilePin(${pinLength})">Set PIN</button>
            </div>
        </div>`;
}

async function saveProfile() {
    const body = {
        first_name: document.getElementById('profFirstName').value.trim(),
        last_name: document.getElementById('profLastName').value.trim(),
        email: document.getElementById('profEmail').value.trim()
    };
    if (!body.email) { toast('Email is required', 'error'); return; }
    const d = await api('PUT', '/profile', body);
    if (d.success) {
        toast('Profile updated!');
        // Update local user data
        currentUser = { ...currentUser, ...d.data };
        localStorage.setItem('user', JSON.stringify(currentUser));
    } else toast(d.error || 'Failed to update profile', 'error');
}

async function saveProfilePassword() {
    const pw = document.getElementById('profPassword').value;
    const pwc = document.getElementById('profPasswordConfirm').value;
    if (!pw) { toast('Please enter a new password', 'error'); return; }
    if (pw !== pwc) { toast('Passwords do not match', 'error'); return; }
    if (pw.length < 4) { toast('Password must be at least 4 characters', 'error'); return; }
    const d = await api('PUT', '/profile', { password: pw });
    if (d.success) {
        toast('Password updated!');
        document.getElementById('profPassword').value = '';
        document.getElementById('profPasswordConfirm').value = '';
    } else toast(d.error || 'Failed to update password', 'error');
}

async function saveProfilePin(pinLength) {
    const pin = document.getElementById('profPin').value;
    if (!pin || pin.length !== pinLength) { toast(`PIN must be exactly ${pinLength} digits`, 'error'); return; }
    if (!/^\d+$/.test(pin)) { toast('PIN must contain only digits', 'error'); return; }
    const d = await api('PUT', '/auth/pin', { pin });
    if (d.success) {
        toast('PIN updated!');
        document.getElementById('profPin').value = '';
        loadProfileView(); // Refresh to show updated PIN status
    } else toast(d.error || 'Failed to set PIN', 'error');
}

// ──── User Stats View (P9-01) ────
async function loadStatsView() {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="section-header"><h2 class="section-title">My Stats</h2></div><div id="statsContainer"><div class="spinner"></div></div>';
    const res = await api('GET', '/profile/stats');
    if (!res.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Failed to load stats</div></div>'; return; }
    const s = res.data;
    const genres = (s.top_genres || []).map(g => `<span class="genre-chip">${g.name} <small>(${g.count})</small></span>`).join('');
    const shows = (s.top_shows || []).map(sh => `<li>${sh.title} <small>(${sh.count} episodes)</small></li>`).join('');
    const perfs = (s.top_performers || []).map(p => `<li>${p.name} <small>(${p.count})</small></li>`).join('');
    // Build heatmap (simple text-based)
    const heatmap = s.heatmap || [];
    const dayNames = ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'];
    let heatHTML = '<div class="stats-heatmap">';
    for (let d = 0; d < 7; d++) {
        heatHTML += `<div class="heatmap-row"><span class="heatmap-day">${dayNames[d]}</span>`;
        for (let h = 0; h < 24; h++) {
            const cell = heatmap.find(c => c.day === d && c.hour === h);
            const count = cell ? cell.count : 0;
            const intensity = Math.min(count / 5, 1);
            heatHTML += `<div class="heatmap-cell" style="opacity:${0.15 + intensity * 0.85}" title="${dayNames[d]} ${h}:00 - ${count} plays"></div>`;
        }
        heatHTML += '</div>';
    }
    heatHTML += '</div>';
    document.getElementById('statsContainer').innerHTML = `
        <div class="stats-grid">
            <div class="stat-card"><div class="stat-value">${s.total_watch_hours || 0}</div><div class="stat-label">Hours Watched</div></div>
            <div class="stat-card"><div class="stat-value">${s.items_watched || 0}</div><div class="stat-label">Items Completed</div></div>
            <div class="stat-card"><div class="stat-value">${s.average_rating ? s.average_rating.toFixed(1) : '—'}</div><div class="stat-label">Avg Rating Given</div></div>
        </div>
        <div class="stats-section"><h3>Top Genres</h3><div class="genre-chips">${genres || '<span style="color:#5a6a7f">No data yet</span>'}</div></div>
        <div class="stats-section"><h3>Top Shows</h3><ol class="stats-list">${shows || '<li style="color:#5a6a7f">No data yet</li>'}</ol></div>
        <div class="stats-section"><h3>Top Performers</h3><ol class="stats-list">${perfs || '<li style="color:#5a6a7f">No data yet</li>'}</ol></div>
        <div class="stats-section"><h3>Watch Activity Heatmap</h3>${heatHTML}</div>
        <div style="text-align:center;margin-top:24px;"><button class="btn-secondary" onclick="navigate('wrapped','${new Date().getFullYear()}')">View ${new Date().getFullYear()} Wrapped</button></div>`;
}

// ──── Genre Hub View (P9-04) ────
async function loadGenreHubView(slug) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner" style="margin:60px auto;"></div>';
    const res = await api('GET', '/discover/genre/' + encodeURIComponent(slug));
    if (!res.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Genre not found</div></div>'; return; }
    const g = res.data;
    const subGenres = (g.sub_genres || []).map(sg => `<span class="genre-chip" onclick="navigate('genre','${sg}')">${sg}</span>`).join('');
    mc.innerHTML = `
        <div class="genre-hub-header"><h1 class="genre-hub-title">${g.name}</h1><span class="genre-hub-count">${g.total} items</span></div>
        ${subGenres ? '<div class="genre-sub-tags">' + subGenres + '</div>' : ''}
        <div class="media-grid">${g.items.map(renderMediaCard).join('')}</div>`;
}

// ──── Decade Hub View (P9-04) ────
async function loadDecadeHubView(year) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner" style="margin:60px auto;"></div>';
    const res = await api('GET', '/discover/decade/' + encodeURIComponent(year));
    if (!res.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">No items found</div></div>'; return; }
    const d = res.data;
    mc.innerHTML = `
        <div class="genre-hub-header"><h1 class="genre-hub-title">The ${d.decade}</h1><span class="genre-hub-count">${d.total} items</span></div>
        <div class="media-grid">${d.items.map(renderMediaCard).join('')}</div>`;
}

// ──── Year-in-Review / Wrapped (P9-06) ────
async function loadWrappedView(year) {
    if (!year) year = new Date().getFullYear();
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner" style="margin:60px auto;"></div>';
    const res = await api('GET', '/profile/wrapped/' + year);
    if (!res.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">No data for ' + year + '</div></div>'; return; }
    const w = res.data;
    const topMovies = (w.top_movies || []).map(m => `<div class="wrapped-item">${m.poster_path ? '<img src="'+posterSrc(m.poster_path,'')+'" class="wrapped-poster">' : ''}<span>${m.title}</span></div>`).join('');
    const topShows = (w.top_shows || []).map(s => `<div class="wrapped-item">${s.poster_path ? '<img src="'+posterSrc(s.poster_path,'')+'" class="wrapped-poster">' : ''}<span>${s.title} <small>(${s.episodes} eps)</small></span></div>`).join('');
    mc.innerHTML = `
        <div class="wrapped-container">
            <h1 class="wrapped-title">${year} Wrapped</h1>
            <div class="stats-grid">
                <div class="stat-card wrapped-card"><div class="stat-value">${w.total_hours || 0}</div><div class="stat-label">Hours Watched</div></div>
                <div class="stat-card wrapped-card"><div class="stat-value">${w.items_watched || 0}</div><div class="stat-label">Items Watched</div></div>
                <div class="stat-card wrapped-card"><div class="stat-value">${w.longest_binge_hours || 0}h</div><div class="stat-label">Longest Binge</div></div>
            </div>
            <div class="stats-section"><h3>Top Genres</h3><div class="genre-chips">${(w.top_genres||[]).map(g=>'<span class="genre-chip">'+g+'</span>').join('')}</div></div>
            <div class="stats-section"><h3>Top Movies</h3><div class="wrapped-grid">${topMovies||'<span style="color:#5a6a7f">No movies watched</span>'}</div></div>
            <div class="stats-section"><h3>Top Shows</h3><div class="wrapped-grid">${topShows||'<span style="color:#5a6a7f">No shows watched</span>'}</div></div>
        </div>`;
}

// ──── Content Requests (P9-07) ────
async function loadContentRequestsView() {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="section-header"><h2 class="section-title">Content Requests</h2><button class="btn-primary btn-sm" onclick="showNewRequestDialog()">+ New Request</button></div><div id="requestsList"><div class="spinner"></div></div>';
    const res = await api('GET', '/requests/mine');
    if (!res.success) { document.getElementById('requestsList').innerHTML = '<div class="empty-state"><div class="empty-state-title">Failed to load</div></div>'; return; }
    const items = res.data;
    if (items.length === 0) {
        document.getElementById('requestsList').innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#128269;</div><div class="empty-state-title">No requests yet</div><p>Request movies or shows you would like added</p></div>';
        return;
    }
    document.getElementById('requestsList').innerHTML = `<div class="requests-grid">${items.map(r => `<div class="request-card">
        <div class="request-poster">${r.poster_url ? '<img src="'+r.poster_url+'">' : '<span style="font-size:2rem;">&#127916;</span>'}</div>
        <div class="request-info">
            <div class="request-title">${r.title} ${r.year ? '('+r.year+')' : ''}</div>
            <div class="request-type">${r.media_type}</div>
            <div class="request-status status-${r.status}">${r.status}</div>
            ${r.admin_note ? '<div class="request-note">'+r.admin_note+'</div>' : ''}
        </div>
    </div>`).join('')}</div>`;
}

async function showNewRequestDialog() {
    const title = prompt('Enter movie or show title:');
    if (!title) return;
    const mediaType = confirm('Is this a TV show? (OK = TV, Cancel = Movie)') ? 'tv' : 'movie';
    const res = await api('POST', '/requests', { title: title, media_type: mediaType });
    if (res.success) { toast('Request submitted!'); loadContentRequestsView(); }
    else toast(res.error || 'Failed to submit request', 'error');
}

// ──────────────────── Artwork Picker ────────────────────

function browseArtworkFromEdit(type) {
    const modal = document.getElementById('editModalContainer');
    if (modal && modal.classList.contains('tabbed')) {
        switchEditTab('artwork');
        return;
    }
    const mediaId = document.getElementById('editMediaId').value;
    if (!mediaId) { toast('No media item loaded', 'error'); return; }
    openArtworkPicker(mediaId, type);
}

function identifyFromEdit() {
    const mediaId = document.getElementById('editMediaId').value;
    if (!mediaId) { toast('No media item loaded', 'error'); return; }
    closeEditModal();
    identifyMedia(mediaId);
}

async function openArtworkPicker(mediaId, type) {
    const res = await api('GET', `/media/${mediaId}/artwork`);
    if (!res.success || !res.data) {
        toast(res.error || 'No artwork available from cache server', 'error');
        return;
    }
    const urls = type === 'poster' ? (res.data.posters || []) :
                 type === 'backdrop' ? (res.data.backdrops || []) : (res.data.logos || []);
    if (urls.length === 0) {
        toast(`No ${type} images available`, 'error');
        return;
    }
    showArtworkPickerModal(mediaId, type, urls);
}

function showArtworkPickerModal(mediaId, type, urls) {
    let existing = document.getElementById('artworkPickerOverlay');
    if (existing) existing.remove();

    const overlay = document.createElement('div');
    overlay.id = 'artworkPickerOverlay';
    overlay.className = 'artwork-picker-overlay';

    const label = type.charAt(0).toUpperCase() + type.slice(1);
    let grid = '';
    urls.forEach((url, i) => {
        grid += `<div class="artwork-thumb" data-idx="${i}" onclick="selectArtwork(this,'${mediaId}','${type}','${url.replace(/'/g,"\\'")}')">
            <img src="${url}" loading="lazy" alt="${label} ${i+1}">
            <span class="artwork-source">${extractArtworkSource(url)}</span>
        </div>`;
    });

    overlay.innerHTML = `
        <div class="artwork-picker-modal">
            <div class="artwork-picker-header">
                <h2>Choose ${label} (${urls.length} available)</h2>
                <button class="artwork-picker-close" onclick="this.closest('.artwork-picker-overlay').remove()">&times;</button>
            </div>
            <div class="artwork-picker-grid">${grid}</div>
        </div>`;
    document.body.appendChild(overlay);
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) overlay.remove();
    });
}

function extractArtworkSource(url) {
    if (url.includes('tmdb.org') || url.includes('themoviedb.org')) return 'TMDB';
    if (url.includes('fanart.tv')) return 'Fanart.tv';
    if (url.includes('thetvdb.com')) return 'TVDB';
    if (url.includes('anilist')) return 'AniList';
    return 'Source';
}

async function selectArtwork(el, mediaId, type, url) {
    document.querySelectorAll('.artwork-thumb.selected').forEach(t => t.classList.remove('selected'));
    el.classList.add('selected');
    const res = await api('PUT', `/media/${mediaId}/artwork`, { type: type, url: url });
    if (res.success) {
        toast(`${type.charAt(0).toUpperCase()+type.slice(1)} updated!`);
        const overlay = document.getElementById('artworkPickerOverlay');
        if (overlay) overlay.remove();
        loadMediaDetail(mediaId);
    } else {
        toast(res.error || 'Failed to update artwork', 'error');
    }
}

// ──────────────────── Collection Artwork Picker ────────────────────

async function openCollectionArtworkPicker(collId, type) {
    const res = await api('GET', `/collections/${collId}/artwork`);
    if (!res.success || !res.data) {
        toast(res.error || 'No artwork available from cache server', 'error');
        return;
    }
    const urls = type === 'poster' ? (res.data.posters || []) : (res.data.backdrops || []);
    if (urls.length === 0) {
        toast(`No ${type} images available`, 'error');
        return;
    }
    showCollectionArtworkPickerModal(collId, type, urls);
}

function showCollectionArtworkPickerModal(collId, type, urls) {
    let existing = document.getElementById('artworkPickerOverlay');
    if (existing) existing.remove();

    const overlay = document.createElement('div');
    overlay.id = 'artworkPickerOverlay';
    overlay.className = 'artwork-picker-overlay';

    const label = type.charAt(0).toUpperCase() + type.slice(1);
    let grid = '';
    urls.forEach((url, i) => {
        grid += `<div class="artwork-thumb" data-idx="${i}" onclick="selectCollectionArtwork(this,'${collId}','${type}','${url.replace(/'/g,"\\'")}')">
            <img src="${url}" loading="lazy" alt="${label} ${i+1}">
            <span class="artwork-source">${extractArtworkSource(url)}</span>
        </div>`;
    });

    overlay.innerHTML = `
        <div class="artwork-picker-modal">
            <div class="artwork-picker-header">
                <h2>Choose Collection ${label} (${urls.length} available)</h2>
                <button class="artwork-picker-close" onclick="this.closest('.artwork-picker-overlay').remove()">&times;</button>
            </div>
            <div class="artwork-picker-grid">${grid}</div>
        </div>`;
    document.body.appendChild(overlay);
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) overlay.remove();
    });
}

async function selectCollectionArtwork(el, collId, type, url) {
    document.querySelectorAll('.artwork-thumb.selected').forEach(t => t.classList.remove('selected'));
    el.classList.add('selected');
    const res = await api('PUT', `/collections/${collId}/artwork`, { type: type, url: url });
    if (res.success) {
        toast(`Collection ${type.charAt(0).toUpperCase()+type.slice(1)} updated!`);
        const overlay = document.getElementById('artworkPickerOverlay');
        if (overlay) overlay.remove();
        loadCollectionDetailView(collId);
    } else {
        toast(res.error || 'Failed to update collection artwork', 'error');
    }
}