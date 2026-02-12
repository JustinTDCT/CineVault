
// ──── Auth ────
let fastLoginConfig = null;
let fastLoginUsers = [];
let selectedFastUser = null;
let pinLength = 4;

async function checkAuth() {
    // Always check if first-run setup is needed (handles stale tokens after DB reset)
    try {
        const setupRes = await fetch(API + '/setup/check');
        const setupData = await setupRes.json();
        if (setupData.success && setupData.data && setupData.data.setup_required) {
            localStorage.removeItem('token');
            localStorage.removeItem('user');
            document.getElementById('loginModal').classList.remove('active');
            document.getElementById('fastLoginOverlay').classList.remove('active');
            document.getElementById('setupOverlay').classList.add('active');
            return;
        }
    } catch(e) { /* setup check failed, continue normally */ }

    const token = localStorage.getItem('token');
    const user = localStorage.getItem('user');
    if (token && user) {
        currentUser = JSON.parse(user);
        document.getElementById('loginModal').classList.remove('active');
        document.getElementById('fastLoginOverlay').classList.remove('active');
        document.getElementById('setupOverlay').classList.remove('active');
        document.getElementById('userAvatar').textContent = currentUser.username[0].toUpperCase();

        // Play intro if enabled and not yet played this session
        if (!sessionStorage.getItem('intro_played')) {
            if (!fastLoginConfig) {
                try {
                    const res = await fetch(API + '/auth/fast-login/settings');
                    fastLoginConfig = await res.json();
                } catch(e) { fastLoginConfig = { success: false }; }
            }
            const introEnabled = fastLoginConfig && fastLoginConfig.success && fastLoginConfig.data && fastLoginConfig.data.login_intro_enabled === 'true';
            if (introEnabled) {
                const played = await playLoginIntro(fastLoginConfig.data.login_intro_muted === 'true');
                if (played) sessionStorage.setItem('intro_played', '1');
            }
        }

        // Fetch overlay display preferences
        fetchOverlayPrefs();

        // If we haven't picked a profile yet this session (master user), show picker
        if (!sessionStorage.getItem('profile_picked') && !currentUser.parent_user_id) {
            showHouseholdPicker();
        } else {
            loadHomeView();
        }
        loadSidebarCounts();
        connectWS();
        // Init Chromecast when SDK loads (P14-02)
        window['__onGCastApiAvailable'] = function(isAvailable) { if (isAvailable) initCast(); };
    } else {
        try {
            const res = await fetch(API + '/auth/fast-login/settings');
            fastLoginConfig = await res.json();
        } catch { fastLoginConfig = { success: false }; }

        if (fastLoginConfig && fastLoginConfig.success && fastLoginConfig.data && fastLoginConfig.data.fast_login_enabled === 'true') {
            pinLength = parseInt(fastLoginConfig.data.fast_login_pin_length) || 4;
            showFastLogin();
        } else {
            document.getElementById('loginModal').classList.add('active');
        }
    }
}

// ──── Fast Login ────
async function showFastLogin() {
    document.getElementById('fastLoginOverlay').classList.add('active');
    document.getElementById('loginModal').classList.remove('active');
    try {
        const res = await fetch(API + '/auth/fast-login/users');
        const data = await res.json();
        fastLoginUsers = data.success ? (data.data || []) : [];
    } catch { fastLoginUsers = []; }
    fastLoginShowUsers();
}

function userColor(name) {
    const colors = [
        ['#6c9a8b','#8fb8a8'],['#9a8b6c','#b8a88f'],['#6c7d9a','#8f9db8'],
        ['#9a6c7d','#b88f9d'],['#7d9a6c','#9db88f'],['#8b6c9a','#a88fb8'],
        ['#6c9a9a','#8fb8b8'],['#9a9a6c','#b8b88f']
    ];
    let hash = 0;
    for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
    return colors[Math.abs(hash) % colors.length];
}

function fastLoginShowUsers() {
    const grid = document.getElementById('fastLoginGrid');
    const pinEntry = document.getElementById('pinEntryContainer');
    const back = document.getElementById('fastLoginBack');
    const title = document.getElementById('fastLoginTitle');
    const fallback = document.querySelector('.fast-login-fallback');
    grid.style.display = 'flex';
    pinEntry.classList.remove('active');
    back.style.display = 'none';
    title.textContent = 'Select User';
    if (fallback) fallback.style.display = '';
    selectedFastUser = null;

    grid.innerHTML = fastLoginUsers.map(u => {
        const dn = u.display_name || u.username;
        const initial = dn[0].toUpperCase();
        const [bg, circle] = userColor(dn);
        let badges = '';
        if (u.has_pin) badges += '<span class="fast-login-badge pin-set">&#128274;</span>';
        if (u.role === 'admin') badges += '<span class="fast-login-badge admin">&#128081;</span>';
        return '<div class="fast-login-user" onclick="fastLoginSelectUser(\'' + u.id + '\')" style="background:linear-gradient(145deg, ' + bg + '33 0%, ' + bg + '11 100%);border-color:' + bg + '44;">' +
            '<div class="fast-login-avatar" style="background:radial-gradient(circle, ' + circle + ' 0%, ' + bg + ' 100%);">' + initial +
            '<div class="fast-login-badges" style="position:absolute;bottom:-4px;left:0;right:0;">' + badges + '</div></div>' +
            '<div class="fast-login-user-name">' + dn + '</div></div>';
    }).join('');
}

function fastLoginSelectUser(userId) {
    selectedFastUser = fastLoginUsers.find(u => u.id === userId);
    if (!selectedFastUser) return;
    if (!selectedFastUser.has_pin) { fastLoginShowStandard(); return; }

    document.getElementById('fastLoginGrid').style.display = 'none';
    document.getElementById('pinEntryContainer').classList.add('active');
    document.getElementById('fastLoginBack').style.display = 'flex';
    document.getElementById('fastLoginTitle').textContent = 'Enter PIN';
    const fallback = document.querySelector('.fast-login-fallback');
    if (fallback) fallback.style.display = 'none';

    const dn = selectedFastUser.display_name || selectedFastUser.username;
    const [bg, circle] = userColor(dn);
    document.getElementById('pinEntryAvatar').style.background = 'radial-gradient(circle, ' + circle + ' 0%, ' + bg + ' 100%)';
    document.getElementById('pinEntryAvatar').textContent = dn[0].toUpperCase();
    document.getElementById('pinEntryName').textContent = dn;
    document.getElementById('pinError').textContent = '';

    let boxesHtml = '';
    for (let i = 0; i < pinLength; i++) boxesHtml += '<div class="pin-box' + (i === 0 ? ' active' : '') + '" id="pinBox' + i + '"></div>';
    document.getElementById('pinBoxes').innerHTML = boxesHtml;

    const hi = document.getElementById('pinHiddenInput');
    hi.value = '';
    hi.maxLength = pinLength;
    setTimeout(function(){ hi.focus(); }, 100);
    document.getElementById('pinBoxes').onclick = function(){ hi.focus(); };
}

document.getElementById('pinHiddenInput').addEventListener('input', async function() {
    const val = this.value.replace(/\D/g, '').substring(0, pinLength);
    this.value = val;
    for (let i = 0; i < pinLength; i++) {
        const box = document.getElementById('pinBox' + i);
        if (!box) continue;
        box.textContent = i < val.length ? '\u25CF' : '';
        box.className = 'pin-box' + (i < val.length ? ' filled' : '') + (i === val.length ? ' active' : '');
    }
    if (val.length === pinLength && selectedFastUser) {
        document.getElementById('pinError').textContent = '';
        try {
            const res = await fetch(API + '/auth/fast-login', {
                method: 'POST', headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ user_id: selectedFastUser.id, pin: val })
            });
            const data = await res.json();
            if (data.success) {
                localStorage.setItem('token', data.data.token);
                localStorage.setItem('user', JSON.stringify(data.data.user));
                checkAuth();
            } else {
                document.getElementById('pinError').textContent = data.error || 'Invalid PIN';
                this.value = '';
                for (let i = 0; i < pinLength; i++) {
                    const box = document.getElementById('pinBox' + i);
                    if (box) { box.textContent = ''; box.className = 'pin-box' + (i === 0 ? ' active' : ''); }
                }
            }
        } catch {
            document.getElementById('pinError').textContent = 'Connection error';
            this.value = '';
        }
    }
});

document.getElementById('pinHiddenInput').addEventListener('keydown', function(e) {
    if (e.key === 'Escape') fastLoginShowUsers();
});

function fastLoginShowStandard() {
    document.getElementById('fastLoginOverlay').classList.remove('active');
    document.getElementById('loginModal').classList.add('active');
    if (selectedFastUser) {
        document.getElementById('loginUsername').value = selectedFastUser.username;
        document.getElementById('loginPassword').focus();
    }
}

// ──── Login Intro Video ────
function playLoginIntro(wantMuted) {
    return new Promise(function(resolve) {
        const overlay = document.getElementById('introOverlay');
        const video = document.getElementById('introVideo');
        overlay.classList.add('active');
        video.playbackRate = 1.5;
        video.currentTime = 0;
        video.muted = !!wantMuted;

        function onEnded() {
            overlay.classList.add('fade-out');
            setTimeout(function() {
                overlay.classList.remove('active', 'fade-out');
                cleanup();
                resolve(true);
            }, 800);
        }

        function onError() {
            overlay.classList.remove('active');
            cleanup();
            resolve(false);
        }

        function cleanup() {
            video.onended = null;
            video.onerror = null;
        }

        video.onended = onEnded;
        video.onerror = onError;

        video.play().catch(function() {
            // Autoplay with audio blocked — retry muted (browsers always allow muted autoplay)
            if (!video.muted) {
                video.muted = true;
                video.play().catch(function() {
                    // Even muted failed — skip
                    overlay.classList.remove('active');
                    cleanup();
                    resolve(false);
                });
            } else {
                overlay.classList.remove('active');
                cleanup();
                resolve(false);
            }
        });
    });
}

// ──── Setup Form ────
document.getElementById('setupForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const msgDiv = document.getElementById('setupMessage');
    const body = {
        username: document.getElementById('setupUsername').value.trim(),
        first_name: document.getElementById('setupFirstName').value.trim(),
        last_name: document.getElementById('setupLastName').value.trim(),
        email: document.getElementById('setupEmail').value.trim(),
        password: document.getElementById('setupPassword').value,
        pin: document.getElementById('setupPin').value.trim()
    };
    try {
        const res = await fetch(API + '/setup', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });
        const data = await res.json();
        if (data.success) {
            localStorage.setItem('token', data.data.token);
            localStorage.setItem('user', JSON.stringify(data.data.user));
            document.getElementById('setupOverlay').classList.remove('active');
            checkAuth();
        } else {
            msgDiv.innerHTML = '<div class="message error">' + (data.error || 'Setup failed') + '</div>';
        }
    } catch {
        msgDiv.innerHTML = '<div class="message error">Connection error</div>';
    }
});

document.getElementById('loginForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const msgDiv = document.getElementById('authMessage');
    try {
        const data = await api('POST', '/auth/login', { username: document.getElementById('loginUsername').value, password: document.getElementById('loginPassword').value });
        if (data.success) { localStorage.setItem('token', data.data.token); localStorage.setItem('user', JSON.stringify(data.data.user)); checkAuth(); }
        else msgDiv.innerHTML='<div class="message error">'+(data.error||'Login failed')+'</div>';
    } catch { msgDiv.innerHTML='<div class="message error">Connection error</div>'; }
});

// ──── User Avatar Dropdown ────
function doLogout() {
    localStorage.clear(); sessionStorage.removeItem('intro_played'); sessionStorage.removeItem('profile_picked'); currentUser=null;
    if(ws) ws.close();
    document.getElementById('fastLoginOverlay').classList.remove('active');
    document.getElementById('loginModal').classList.remove('active');
    closeHouseholdPicker();
    closeUserDropdown();
    checkAuth();
}
document.getElementById('udLogoutBtn').addEventListener('click', doLogout);

document.getElementById('userAvatar').addEventListener('click', (e) => {
    e.stopPropagation();
    const dd = document.getElementById('userDropdown');
    if (dd.classList.contains('open')) { closeUserDropdown(); return; }
    // Populate dropdown header
    if (currentUser) {
        const nameEl = document.getElementById('udName');
        const fn = currentUser.first_name || '';
        const ln = currentUser.last_name || '';
        nameEl.textContent = (fn + ' ' + ln).trim() || currentUser.username;
        document.getElementById('udRole').textContent = currentUser.role;
        document.getElementById('udSettingsItem').style.display = currentUser.role === 'admin' ? '' : 'none';
        // Show Manage Profiles for master users (no parent_user_id)
        const isMaster = !currentUser.parent_user_id;
        document.getElementById('udManageProfilesItem').style.display = isMaster ? '' : 'none';
    }
    dd.classList.add('open');
});

function closeUserDropdown() {
    document.getElementById('userDropdown').classList.remove('open');
}
document.addEventListener('click', (e) => {
    if (!e.target.closest('.user-menu')) closeUserDropdown();
});

// ──── Sidebar Libraries ────
async function loadSidebarCounts() {
    try {
        const data = await api('GET', '/libraries');
        if (!data.success) return;
        allLibraries = data.data || [];
        const nav = document.getElementById('librariesNav');
        // Keep the label, rebuild items
        nav.innerHTML = '<div class="nav-label">Libraries</div>';
        for (const lib of allLibraries) {
            const icon = MEDIA_ICONS[lib.media_type] || '&#128218;';
            const item = document.createElement('div');
            item.className = 'nav-item nav-item-lib';
            item.dataset.view = 'library';
            item.dataset.id = lib.id;
            item.innerHTML = `<span class="nav-icon">${icon}</span><span class="nav-lib-info"><span class="nav-lib-name">${lib.name}</span><span class="nav-lib-count" id="badge-lib-${lib.id}"></span></span><span class="nav-lib-dots" data-lib-id="${lib.id}" title="Library options">&#8943;</span>`;
            item.querySelector('.nav-lib-info').addEventListener('click', () => navigate('library', lib.id));
            item.querySelector('.nav-icon').addEventListener('click', () => navigate('library', lib.id));
            const dotsBtn = item.querySelector('.nav-lib-dots');
            dotsBtn.addEventListener('click', (e) => { e.stopPropagation(); openLibCtxMenu(lib.id, dotsBtn); });
            nav.appendChild(item);
        }
        // Load counts
        for (const lib of allLibraries) {
            const countData = await api('GET', '/libraries/' + lib.id + '/media');
            if (countData.success) {
                const badge = document.getElementById('badge-lib-' + lib.id);
                if (badge) {
                    const total = countData.data.total || 0;
                    badge.textContent = total > 0 ? total.toLocaleString() + ' items' : '';
                }
            }
        }
        // Show Manage section for admins and load duplicate count badge
        if (currentUser && currentUser.role === 'admin') {
            const manageNav = document.getElementById('manageNav');
            if (manageNav) manageNav.style.display = '';
            try {
                const dupData = await api('GET', '/duplicates/count');
                const dupBadge = document.getElementById('dupBadge');
                if (dupData.success && dupData.data && dupData.data.count > 0 && dupBadge) {
                    dupBadge.textContent = dupData.data.count;
                    dupBadge.style.display = '';
                } else if (dupBadge) {
                    dupBadge.style.display = 'none';
                }
            } catch {}
        }
    } catch {}
    // Load version footer
    loadSidebarVersion();
}

async function loadSidebarVersion() {
    try {
        const d = await api('GET', '/status');
        const ver = d.success && d.data ? d.data.version : '';
        const el = document.getElementById('sidebarFooter');
        if (el && ver) el.innerHTML = `<div class="sidebar-version">v${ver}</div><div class="sidebar-credit">Designed by 21 Mexican Jumping Llamas</div>`;
    } catch {}
}

// ──── Sidebar Library Context Menu ────
function openLibCtxMenu(libId, dotsEl) {
    const menu = document.getElementById('libCtxMenu');
    const wasOpen = menu.classList.contains('open') && menu.dataset.libId === libId;
    closeLibCtxMenu();
    if (wasOpen) return;

    const isAdmin = currentUser && currentUser.role === 'admin';
    let html = '';
    if (isAdmin) html += `<div class="lib-ctx-item" onclick="showEditLibraryForm('${libId}')"><span class="ctx-icon">&#9998;</span> Edit Library</div><div class="lib-ctx-sep"></div>`;
    html += `<div class="lib-ctx-item" onclick="sidebarScanLibrary('${libId}')"><span class="ctx-icon">&#128269;</span> Scan Library</div>`;
    html += `<div class="lib-ctx-item" onclick="sidebarMetadataRefresh('${libId}')"><span class="ctx-icon">&#8635;</span> Metadata Refresh</div>`;
    html += `<div class="lib-ctx-item" onclick="sidebarRehashPhash('${libId}')"><span class="ctx-icon">&#128274;</span> Rehash pHash</div>`;
    menu.innerHTML = html;
    menu.dataset.libId = libId;

    // Position relative to the dots button
    const rect = dotsEl.getBoundingClientRect();
    menu.style.left = (rect.right - 4) + 'px';
    menu.style.top = rect.top + 'px';
    menu.classList.add('open');
    dotsEl.classList.add('active');

    // Adjust if menu goes off-screen bottom
    requestAnimationFrame(() => {
        const menuRect = menu.getBoundingClientRect();
        if (menuRect.bottom > window.innerHeight - 10) {
            menu.style.top = Math.max(10, window.innerHeight - menuRect.height - 10) + 'px';
        }
    });
}
function closeLibCtxMenu() {
    const menu = document.getElementById('libCtxMenu');
    menu.classList.remove('open');
    menu.dataset.libId = '';
    document.querySelectorAll('.nav-lib-dots.active').forEach(d => d.classList.remove('active'));
}
document.addEventListener('click', (e) => {
    if (!e.target.closest('.nav-lib-dots') && !e.target.closest('.lib-ctx-menu')) closeLibCtxMenu();
});

function sidebarScanLibrary(libId) {
    closeLibCtxMenu();
    toast('Scan started...');
    api('POST', '/libraries/' + libId + '/scan').then(data => {
        if (data.success) {
            if (data.data.job_id) { /* progress via WebSocket */ }
            else { toast(`Scan: ${data.data.files_added} added, ${data.data.files_found} total`); loadSidebarCounts(); }
        } else toast('Scan failed: ' + (data.error || 'Unknown'), 'error');
    }).catch(e => toast('Scan error: ' + e.message, 'error'));
}

function sidebarMetadataRefresh(libId) {
    closeLibCtxMenu();
    const lib = allLibraries.find(l => l.id === libId);
    if (lib && !lib.retrieve_metadata) { toast('Metadata retrieval is disabled for this library', 'error'); return; }
    toast('Metadata refresh started — clearing & re-matching unlocked items...');
    api('POST', '/libraries/' + libId + '/refresh-metadata').then(data => {
        if (data.success) toast('Metadata refresh queued');
        else toast('Failed: ' + (data.error || 'Unknown'), 'error');
    }).catch(e => toast('Error: ' + e.message, 'error'));
}

function sidebarRehashPhash(libId) {
    closeLibCtxMenu();
    toast('pHash computation started...');
    api('POST', '/libraries/' + libId + '/phash').then(data => {
        if (data.success) toast('pHash job queued — duplicates will be analyzed');
        else toast('Failed: ' + (data.error || 'Unknown'), 'error');
    }).catch(e => toast('Error: ' + e.message, 'error'));
}

async function showEditLibraryForm(libId) {
    closeLibCtxMenu();
    const data = await api('GET', '/libraries/' + libId);
    if (!data.success) { toast('Failed to load library', 'error'); return; }
    const lib = data.data;
    const mc = document.getElementById('mainContent');
    const types = Object.entries(MEDIA_LABELS).map(([k,v])=>`<option value="${k}" ${k===lib.media_type?'selected':''}>${v}</option>`).join('');
    const folders = (lib.folders && lib.folders.length > 0) ? lib.folders : [{ folder_path: lib.path }];
    const folderRows = folders.map((f, i) => `<div class="folder-row" data-idx="${i}">
        <input type="text" class="lib-folder-input" placeholder="/media/folder" style="flex:1;" value="${f.folder_path || ''}">
        <button class="btn-secondary" onclick="openFolderBrowser(${i})" style="white-space:nowrap;padding:8px 12px;font-size:0.8rem;">&#128193; Browse</button>
        ${i > 0 ? '<button class="folder-remove" onclick="removeFolderRow(this)" title="Remove folder">&#10005;</button>' : ''}
    </div>`).join('');

    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Edit Library</h2></div>
    <div style="max-width:560px;">
        <input type="hidden" id="editLibId" value="${lib.id}">
        <div class="form-group"><label>Name</label><input type="text" id="libName" value="${lib.name}"></div>
        <div class="form-group"><label>Media Type</label><select id="libType" disabled>${types}</select>
            <p style="color:#5a6a7f;font-size:0.75rem;margin-top:4px;">Media type cannot be changed after creation</p>
        </div>

        <div class="form-group">
            <label>Folders</label>
            <p style="color:#8a9bae;font-size:0.78rem;margin-bottom:8px;">At least one folder is required. Add more to scan multiple locations.</p>
            <div class="folder-list" id="folderList">${folderRows}</div>
            <button class="folder-add-btn" onclick="addFolderRow()">+ Add Folder</button>
        </div>
        <div id="pathBrowser" style="display:none;margin-bottom:18px;background:rgba(0,0,0,0.3);border:1px solid rgba(0,217,255,0.15);border-radius:12px;padding:14px;max-height:350px;overflow-y:auto;"></div>

        <div id="seasonGroupingOpt" style="${lib.media_type === 'tv_shows' ? '' : 'display:none;'}">
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Group by Season</div>
                    <div class="option-row-desc">Parse SxxExx from filenames to group episodes by season</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="seasonGrouping" value="yes" ${lib.season_grouping?'checked':''}><span>Yes</span></label>
                    <label><input type="radio" name="seasonGrouping" value="no" ${!lib.season_grouping?'checked':''}><span>No</span></label>
                </div>
            </div>
        </div>

        <div id="adultContentOpt" style="${lib.media_type === 'adult_movies' ? '' : 'display:none;'}">
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Content Type</div>
                    <div class="option-row-desc">Clips &amp; Scenes will not retrieve metadata. Movies will scrape from TMDB.</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="adultContentType" value="movies" ${(!lib.adult_content_type || lib.adult_content_type === 'movies')?'checked':''}><span>Movies</span></label>
                    <label><input type="radio" name="adultContentType" value="clips" ${lib.adult_content_type === 'clips'?'checked':''}><span>Clips</span></label>
                </div>
            </div>
        </div>

        <div class="form-group">
            <label>Library Permissions</label>
            <select id="libAccess" onchange="onAccessChange()">
                <option value="everyone" ${lib.access_level==='everyone'?'selected':''}>Everyone</option>
                <option value="select_users" ${lib.access_level==='select_users'?'selected':''}>Select People</option>
                <option value="admin_only" ${lib.access_level==='admin_only'?'selected':''}>Admin Only</option>
            </select>
        </div>
        <div id="userSelectPanel" style="${lib.access_level==='select_users'?'':'display:none;'}margin-bottom:18px;background:rgba(0,0,0,0.3);border:1px solid rgba(0,217,255,0.15);border-radius:12px;padding:14px;">
            <p style="color:#8a9bae;font-size:0.8rem;margin-bottom:10px;">Select users who can access this library:</p>
            <div id="userCheckboxList"><div class="spinner"></div></div>
        </div>

        <div style="margin-bottom:18px;">
            <label style="display:block;margin-bottom:10px;font-weight:600;color:#e5e5e5;">Library Options</label>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Include in Homepage</div>
                    <div class="option-row-desc">Show this library's media on the home screen</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="includeHomepage" value="yes" ${lib.include_in_homepage?'checked':''}><span>Yes</span></label>
                    <label><input type="radio" name="includeHomepage" value="no" ${!lib.include_in_homepage?'checked':''}><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Include in Search</div>
                    <div class="option-row-desc">Allow this library's items to appear in search results</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="includeSearch" value="yes" ${lib.include_in_search?'checked':''}><span>Yes</span></label>
                    <label><input type="radio" name="includeSearch" value="no" ${!lib.include_in_search?'checked':''}><span>No</span></label>
                </div>
            </div>
            <div class="option-row" id="metadataOpt">
                <div class="option-row-info">
                    <div class="option-row-label">Retrieve Metadata</div>
                    <div class="option-row-desc">Auto-populate from TMDB/MusicBrainz/OpenLibrary on scan</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="retrieveMetadata" value="yes" ${lib.retrieve_metadata?'checked':''}><span>Yes</span></label>
                    <label><input type="radio" name="retrieveMetadata" value="no" ${!lib.retrieve_metadata?'checked':''}><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">NFO Import</div>
                    <div class="option-row-desc">Read Kodi/Jellyfin .nfo sidecar files for metadata &amp; provider IDs</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="nfoImport" value="yes" ${lib.nfo_import?'checked':''}><span>Yes</span></label>
                    <label><input type="radio" name="nfoImport" value="no" ${!lib.nfo_import?'checked':''}><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">NFO Export</div>
                    <div class="option-row-desc">Write .nfo sidecar files after metadata is populated</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="nfoExport" value="yes" ${lib.nfo_export?'checked':''}><span>Yes</span></label>
                    <label><input type="radio" name="nfoExport" value="no" ${!lib.nfo_export?'checked':''}><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Prefer Local Artwork</div>
                    <div class="option-row-desc">Use poster/backdrop/logo files found next to media before fetching remote</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="preferLocalArtwork" value="yes" ${lib.prefer_local_artwork!==false?'checked':''}><span>Yes</span></label>
                    <label><input type="radio" name="preferLocalArtwork" value="no" ${lib.prefer_local_artwork===false?'checked':''}><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Scheduled Scan</div>
                    <div class="option-row-desc">Automatically scan this library on a schedule</div>
                </div>
                <select id="editScanInterval" class="form-select" style="width:auto;">
                    <option value="disabled" ${lib.scan_interval==='disabled'||!lib.scan_interval?'selected':''}>Disabled</option>
                    <option value="1h" ${lib.scan_interval==='1h'?'selected':''}>Every Hour</option>
                    <option value="6h" ${lib.scan_interval==='6h'?'selected':''}>Every 6 Hours</option>
                    <option value="12h" ${lib.scan_interval==='12h'?'selected':''}>Every 12 Hours</option>
                    <option value="24h" ${lib.scan_interval==='24h'?'selected':''}>Daily</option>
                    <option value="weekly" ${lib.scan_interval==='weekly'?'selected':''}>Weekly</option>
                </select>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Filesystem Watcher</div>
                    <div class="option-row-desc">Watch folders for new/deleted files in real-time</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="watchEnabled" value="yes" ${lib.watch_enabled?'checked':''}><span>Yes</span></label>
                    <label><input type="radio" name="watchEnabled" value="no" ${!lib.watch_enabled?'checked':''}><span>No</span></label>
                </div>
            </div>
        </div>

        <button class="btn-primary" onclick="saveEditLibrary()">Save Changes</button>
        <button class="btn-secondary" style="margin-left:12px;" onclick="loadLibrariesView()">Cancel</button>
    </div>`;

    // Load user checkboxes if select_users
    if (lib.access_level === 'select_users') {
        const usersData = await api('GET', '/users');
        const list = document.getElementById('userCheckboxList');
        const allowed = lib.allowed_users ? lib.allowed_users.map(u => u.toString()) : [];
        if (usersData.success && usersData.data && usersData.data.length > 0) {
            list.innerHTML = usersData.data.filter(u => u.role !== 'admin').map(u =>
                `<label style="display:flex;align-items:center;gap:8px;padding:6px 0;cursor:pointer;color:#e5e5e5;">
                    <input type="checkbox" class="user-perm-cb" value="${u.id}" ${allowed.includes(u.id)?'checked':''}>
                    <span>${u.username}</span>
                    <span style="color:#5a6a7f;font-size:0.75rem;margin-left:auto;">${u.role}</span>
                </label>`
            ).join('');
            if (list.innerHTML === '') list.innerHTML = '<p style="color:#5a6a7f;font-size:0.85rem;">No non-admin users found</p>';
        }
    }
}

async function saveEditLibrary() {
    const id = document.getElementById('editLibId').value;
    const name = document.getElementById('libName').value;
    const media_type = document.getElementById('libType').value;
    const folderInputs = document.querySelectorAll('.lib-folder-input');
    const folders = [...folderInputs].map(i => i.value.trim()).filter(Boolean);
    if (!name || folders.length === 0) { toast('Name and at least one folder required', 'error'); return; }

    const season_grouping = media_type === 'tv_shows' && document.querySelector('input[name="seasonGrouping"]:checked')?.value === 'yes';
    const access_level = document.getElementById('libAccess').value;
    const allowed_users = [...document.querySelectorAll('.user-perm-cb:checked')].map(cb => cb.value);
    const include_in_homepage = document.querySelector('input[name="includeHomepage"]:checked')?.value === 'yes';
    const include_in_search = document.querySelector('input[name="includeSearch"]:checked')?.value === 'yes';
    const retrieve_metadata = document.querySelector('input[name="retrieveMetadata"]:checked')?.value === 'yes';
    const nfo_import = document.querySelector('input[name="nfoImport"]:checked')?.value === 'yes';
    const nfo_export = document.querySelector('input[name="nfoExport"]:checked')?.value === 'yes';
    const prefer_local_artwork = document.querySelector('input[name="preferLocalArtwork"]:checked')?.value === 'yes';
    const scan_interval = document.getElementById('editScanInterval')?.value || 'disabled';
    const watch_enabled = document.querySelector('input[name="watchEnabled"]:checked')?.value === 'yes';

    let adult_content_type = null;
    if (media_type === 'adult_movies') {
        adult_content_type = document.querySelector('input[name="adultContentType"]:checked')?.value || 'movies';
    }

    const d = await api('PUT', '/libraries/' + id, {
        name, path: folders[0], folders, is_enabled: true,
        season_grouping, access_level, allowed_users,
        include_in_homepage, include_in_search, retrieve_metadata,
        nfo_import, nfo_export, prefer_local_artwork, adult_content_type,
        scan_interval, watch_enabled
    });
    if (d.success) { toast('Library updated!'); loadLibrariesView(); loadSidebarCounts(); }
    else toast('Failed: ' + d.error, 'error');
}

