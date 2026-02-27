// Search and library management views
async function loadSearchView(query) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Search: "${query}"</h2></div><div class="media-grid" id="searchGrid"><div class="spinner"></div> Searching...</div>`;
    const data = await api('GET', '/media/search?q=' + encodeURIComponent(query));
    const grid = document.getElementById('searchGrid');
    grid.innerHTML = (data.success && data.data && data.data.length > 0) ? data.data.map(renderMediaCard).join('') : '<div class="empty-state" style="grid-column:1/-1;"><div class="empty-state-title">No results</div></div>';
    enableGridKeyNav(grid);
}

// ──── Libraries ────
async function loadLibrariesView() {
    const mc = document.getElementById('mainContent');
    const isAdmin = currentUser && currentUser.role === 'admin';
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Libraries</h2>${isAdmin?'<button class="btn-primary" onclick="showCreateLibrary()">+ Add Library</button>':''}</div><div id="libList"><div class="spinner"></div> Loading...</div>`;
    const data = await api('GET', '/libraries');
    const div = document.getElementById('libList');
    if (data.success && data.data && data.data.length > 0) {
        div.innerHTML = data.data.map(lib => {
            const accessLabel = {everyone:'Everyone',select_users:'Select People',admin_only:'Admin Only'}[lib.access_level]||'Everyone';
            const accessColor = {everyone:'tag-green',select_users:'tag-purple',admin_only:'tag-red'}[lib.access_level]||'tag-green';
            const folderPaths = (lib.folders && lib.folders.length > 0) ? lib.folders.map(f => f.folder_path).join(', ') : lib.path;
            const folderCount = (lib.folders && lib.folders.length > 1) ? `<span class="tag tag-orange" style="margin-left:6px;">${lib.folders.length} folders</span>` : '';
            let settingsTags = '';
            if (!lib.include_in_homepage) settingsTags += '<span class="tag tag-red" style="margin-left:4px;">Hidden from Home</span>';
            if (!lib.include_in_search) settingsTags += '<span class="tag tag-red" style="margin-left:4px;">Hidden from Search</span>';
            if (!lib.retrieve_metadata) settingsTags += '<span class="tag tag-orange" style="margin-left:4px;">No Metadata</span>';
            if (lib.create_previews === false) settingsTags += '<span class="tag tag-orange" style="margin-left:4px;">No Previews</span>';
            if (lib.create_thumbnails === false) settingsTags += '<span class="tag tag-orange" style="margin-left:4px;">No Thumbnails</span>';
            if (lib.audio_normalization) settingsTags += '<span class="tag tag-cyan" style="margin-left:4px;">Audio Normalization</span>';
            if (lib.media_type === 'adult_movies' && lib.adult_content_type) settingsTags += `<span class="tag tag-purple" style="margin-left:4px;">${lib.adult_content_type === 'clips' ? 'Clips' : 'Movies'}</span>`;
            return `<div class="library-card" id="lib-card-${lib.id}"><div style="flex:1;"><h3>${lib.name}</h3><p style="color:#8a9bae;font-size:0.85rem;"><span class="tag tag-cyan">${MEDIA_LABELS[lib.media_type]||lib.media_type}</span>${lib.season_grouping?'<span class="tag tag-purple" style="margin-left:6px;">Season Grouping</span>':''}<span class="tag ${accessColor}" style="margin-left:6px;">${accessLabel}</span>${folderCount}<span style="margin-left:8px;">${folderPaths}</span></p><div class="lib-settings-tags">${settingsTags}</div><p style="color:#5a6a7f;font-size:0.78rem;margin-top:6px;">${lib.last_scan_at?'Last scan: '+new Date(lib.last_scan_at).toLocaleString():'Never scanned'}</p><div class="scan-progress" id="scan-progress-${lib.id}"><div class="scan-progress-bar"><div class="scan-progress-fill" id="scan-fill-${lib.id}"></div></div><div class="scan-progress-text"><span class="filename" id="scan-file-${lib.id}"></span><span id="scan-count-${lib.id}"></span></div></div></div><div class="library-actions">${isAdmin?`<button class="btn-secondary" id="scan-btn-${lib.id}" onclick="scanLibrary('${lib.id}',this)">&#128269; Scan</button><button class="btn-danger btn-small" onclick="deleteLibrary('${lib.id}')">Delete</button>`:''}</div></div>`;
        }).join('');
    } else div.innerHTML = `<div class="empty-state"><div class="empty-state-icon">&#128218;</div><div class="empty-state-title">No libraries configured</div><p>Create your first library to start organizing media</p></div>`;
}

async function scanLibrary(id, btn) {
    btn.disabled = true; btn.innerHTML = '<span class="spinner"></span>Scanning...';
    const prog = document.getElementById('scan-progress-' + id);
    if (prog) prog.classList.add('active');
    const countEl = document.getElementById('scan-count-' + id);
    if (countEl) countEl.textContent = 'Counting files...';
    try {
        const data = await api('POST', '/libraries/'+id+'/scan');
        if (data.success) {
            if (data.data.job_id) { /* progress handled by WebSocket events */ }
            else {
                // Synchronous scan fallback (no job queue)
                const r = data.data;
                toast(`Scan: ${r.files_added} added, ${r.files_found} total`);
                loadLibrariesView(); loadSidebarCounts();
                btn.disabled = false; btn.innerHTML = '&#128269; Scan';
                if (prog) prog.classList.remove('active');
            }
        } else {
            toast('Scan failed: '+(data.error||'Unknown'), 'error');
            btn.disabled = false; btn.innerHTML = '&#128269; Scan';
            if (prog) prog.classList.remove('active');
        }
    } catch(e) {
        toast('Scan error: '+e.message, 'error');
        btn.disabled = false; btn.innerHTML = '&#128269; Scan';
        if (prog) prog.classList.remove('active');
    }
}

async function deleteLibrary(id) { if (!confirm('Delete this library and all its media?')) return; const d=await api('DELETE','/libraries/'+id); if(d.success){toast('Library deleted');loadLibrariesView();}else toast('Failed: '+d.error,'error'); }

function showCreateLibrary() {
    const mc = document.getElementById('mainContent');
    const types = Object.entries(MEDIA_LABELS).map(([k,v])=>`<option value="${k}">${v}</option>`).join('');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Create Library</h2></div>
    <div style="max-width:560px;">
        <div class="form-group"><label>Name</label><input type="text" id="libName" placeholder="My Movies"></div>
        <div class="form-group"><label>Media Type</label><select id="libType" onchange="onLibTypeChange()">${types}</select></div>

        <div class="form-group">
            <label>Folders</label>
            <p style="color:#8a9bae;font-size:0.78rem;margin-bottom:8px;">At least one folder is required. Add more to scan multiple locations.</p>
            <div class="folder-list" id="folderList">
                <div class="folder-row" data-idx="0">
                    <input type="text" class="lib-folder-input" placeholder="/media/movies" style="flex:1;">
                    <button class="btn-secondary" onclick="openFolderBrowser(0)" style="white-space:nowrap;padding:8px 12px;font-size:0.8rem;">&#128193; Browse</button>
                </div>
            </div>
            <button class="folder-add-btn" onclick="addFolderRow()">+ Add Folder</button>
        </div>
        <div id="pathBrowser" style="display:none;margin-bottom:18px;background:rgba(0,0,0,0.3);border:1px solid rgba(0,217,255,0.15);border-radius:12px;padding:14px;max-height:350px;overflow-y:auto;"></div>

        <div id="seasonGroupingOpt" style="display:none;">
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Group by Season</div>
                    <div class="option-row-desc">Parse SxxExx from filenames to group episodes by season</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="seasonGrouping" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="seasonGrouping" value="no"><span>No</span></label>
                </div>
            </div>
        </div>

        <div id="adultContentOpt" style="display:none;">
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Content Type</div>
                    <div class="option-row-desc">Clips &amp; Scenes will not retrieve metadata. Movies will scrape from TMDB.</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="adultContentType" value="movies" checked><span>Movies</span></label>
                    <label><input type="radio" name="adultContentType" value="clips"><span>Clips</span></label>
                </div>
            </div>
        </div>

        <div class="form-group">
            <label>Library Permissions</label>
            <select id="libAccess" onchange="onAccessChange()">
                <option value="everyone">Everyone</option>
                <option value="select_users">Select People</option>
                <option value="admin_only">Admin Only</option>
            </select>
        </div>
        <div id="userSelectPanel" style="display:none;margin-bottom:18px;background:rgba(0,0,0,0.3);border:1px solid rgba(0,217,255,0.15);border-radius:12px;padding:14px;">
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
                    <label><input type="radio" name="includeHomepage" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="includeHomepage" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Include in Search</div>
                    <div class="option-row-desc">Allow this library's items to appear in search results</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="includeSearch" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="includeSearch" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row" id="metadataOpt">
                <div class="option-row-info">
                    <div class="option-row-label">Retrieve Metadata</div>
                    <div class="option-row-desc">Auto-populate from TMDB/MusicBrainz/OpenLibrary on scan</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="retrieveMetadata" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="retrieveMetadata" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">NFO Import</div>
                    <div class="option-row-desc">Read Kodi/Jellyfin .nfo sidecar files for metadata &amp; provider IDs</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="nfoImport" value="yes"><span>Yes</span></label>
                    <label><input type="radio" name="nfoImport" value="no" checked><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">NFO Export</div>
                    <div class="option-row-desc">Write .nfo sidecar files after metadata is populated</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="nfoExport" value="yes"><span>Yes</span></label>
                    <label><input type="radio" name="nfoExport" value="no" checked><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Prefer Local Artwork</div>
                    <div class="option-row-desc">Use poster/backdrop/logo files found next to media before fetching remote</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="preferLocalArtwork" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="preferLocalArtwork" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Generate Preview Clips</div>
                    <div class="option-row-desc">Create short animated preview clips shown on hover</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="createPreviews" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="createPreviews" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Generate Timeline Thumbnails</div>
                    <div class="option-row-desc">Create sprite sheets used for the player timeline scrubber</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="createThumbnails" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="createThumbnails" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Audio Normalization</div>
                    <div class="option-row-desc">Analyze loudness (EBU R128) and normalize volume during playback</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="audioNormalization" value="yes"><span>Yes</span></label>
                    <label><input type="radio" name="audioNormalization" value="no" checked><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Scheduled Scan</div>
                    <div class="option-row-desc">Automatically scan this library on a schedule</div>
                </div>
                <select id="editScanInterval" class="form-select" style="width:auto;">
                    <option value="disabled" selected>Disabled</option>
                    <option value="1h">Every Hour</option>
                    <option value="6h">Every 6 Hours</option>
                    <option value="12h">Every 12 Hours</option>
                    <option value="24h">Daily</option>
                    <option value="weekly">Weekly</option>
                </select>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Filesystem Watcher</div>
                    <div class="option-row-desc">Watch folders for new/deleted files in real-time</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="watchEnabled" value="yes"><span>Yes</span></label>
                    <label><input type="radio" name="watchEnabled" value="no" checked><span>No</span></label>
                </div>
            </div>
        </div>

        <button class="btn-primary" onclick="createLibrary()">Create Library</button>
        <button class="btn-secondary" style="margin-left:12px;" onclick="loadLibrariesView()">Cancel</button>
    </div>`;
    onLibTypeChange();
}

let activeFolderIdx = null;

function addFolderRow() {
    const list = document.getElementById('folderList');
    const idx = list.children.length;
    const row = document.createElement('div');
    row.className = 'folder-row';
    row.dataset.idx = idx;
    row.innerHTML = `<input type="text" class="lib-folder-input" placeholder="/media/folder" style="flex:1;">
        <button class="btn-secondary" onclick="openFolderBrowser(${idx})" style="white-space:nowrap;padding:8px 12px;font-size:0.8rem;">&#128193; Browse</button>
        <button class="folder-remove" onclick="removeFolderRow(this)" title="Remove folder">&#10005;</button>`;
    list.appendChild(row);
}

function removeFolderRow(btn) {
    const row = btn.closest('.folder-row');
    const list = document.getElementById('folderList');
    if (list.children.length <= 1) { toast('At least one folder is required', 'error'); return; }
    row.remove();
}

function openFolderBrowser(idx) {
    activeFolderIdx = idx;
    const inputs = document.querySelectorAll('.lib-folder-input');
    const currentPath = inputs[idx]?.value || '/media';
    openPathBrowser(currentPath);
}

function onLibTypeChange() {
    const type = document.getElementById('libType').value;
    const seasonOpt = document.getElementById('seasonGroupingOpt');
    const adultOpt = document.getElementById('adultContentOpt');
    if (seasonOpt) seasonOpt.style.display = (type === 'tv_shows') ? '' : 'none';
    if (adultOpt) adultOpt.style.display = (type === 'adult_movies') ? '' : 'none';
}

async function onAccessChange() {
    const val = document.getElementById('libAccess').value;
    const panel = document.getElementById('userSelectPanel');
    if (val === 'select_users') {
        panel.style.display = '';
        const data = await api('GET', '/users');
        const list = document.getElementById('userCheckboxList');
        if (data.success && data.data && data.data.length > 0) {
            list.innerHTML = data.data
                .filter(u => u.role !== 'admin')
                .map(u => `<label style="display:flex;align-items:center;gap:8px;padding:6px 0;cursor:pointer;color:#e5e5e5;">
                    <input type="checkbox" class="user-perm-cb" value="${u.id}">
                    <span>${u.username}</span>
                    <span style="color:#5a6a7f;font-size:0.75rem;margin-left:auto;">${u.role}</span>
                </label>`).join('');
            if (list.innerHTML === '') list.innerHTML = '<p style="color:#5a6a7f;font-size:0.85rem;">No non-admin users found</p>';
        } else {
            list.innerHTML = '<p style="color:#5a6a7f;font-size:0.85rem;">No users found</p>';
        }
    } else {
        panel.style.display = 'none';
    }
}

async function openPathBrowser(path) {
    const browser = document.getElementById('pathBrowser');
    browser.style.display = 'block';
    await loadPathEntries(path || '/media');
}

async function loadPathEntries(path) {
    const browser = document.getElementById('pathBrowser');
    browser.innerHTML = '<div class="spinner" style="margin:10px auto;"></div>';
    const data = await api('GET', '/browse?path=' + encodeURIComponent(path));
    if (!data.success) { browser.innerHTML = '<p style="color:#ff5555;">Failed to browse</p>'; return; }
    const d = data.data;
    let html = `<div style="display:flex;align-items:center;gap:8px;margin-bottom:12px;padding-bottom:10px;border-bottom:1px solid rgba(0,217,255,0.1);">
        <span style="color:#00D9FF;font-size:0.8rem;font-weight:600;">&#128194; ${d.path}</span>
        <button class="btn-secondary" style="margin-left:auto;padding:4px 12px;font-size:0.75rem;" onclick="selectBrowsePath('${d.path}')">&#10003; Select This</button>
    </div>`;
    if (d.parent) {
        html += `<div onclick="loadPathEntries('${d.parent}')" style="display:flex;align-items:center;gap:8px;padding:8px 10px;cursor:pointer;border-radius:8px;color:#8a9bae;font-size:0.85rem;transition:background 0.2s;" onmouseover="this.style.background='rgba(0,217,255,0.08)'" onmouseout="this.style.background='transparent'">&#11168; ..</div>`;
    }
    if (d.entries && d.entries.length > 0) {
        d.entries.forEach(e => {
            html += `<div onclick="loadPathEntries('${e.path}')" style="display:flex;align-items:center;gap:8px;padding:8px 10px;cursor:pointer;border-radius:8px;color:#e5e5e5;font-size:0.85rem;transition:background 0.2s;" onmouseover="this.style.background='rgba(0,217,255,0.08)'" onmouseout="this.style.background='transparent'">&#128193; ${e.name}</div>`;
        });
    } else if (!d.parent) {
        html += '<p style="color:#5a6a7f;font-size:0.8rem;text-align:center;margin:12px 0;">No subdirectories</p>';
    } else {
        html += '<p style="color:#5a6a7f;font-size:0.8rem;text-align:center;margin:12px 0;">Empty folder</p>';
    }
    browser.innerHTML = html;
}

function selectBrowsePath(path) {
    // Multi-folder mode: set the active folder input
    if (activeFolderIdx !== null) {
        const inputs = document.querySelectorAll('.lib-folder-input');
        if (inputs[activeFolderIdx]) {
            inputs[activeFolderIdx].value = path;
        }
        activeFolderIdx = null;
    } else {
        // Fallback for single-path mode (legacy)
        const libPath = document.getElementById('libPath');
        if (libPath) libPath.value = path;
    }
    document.getElementById('pathBrowser').style.display = 'none';
}

async function createLibrary() {
    const name = document.getElementById('libName').value;
    const media_type = document.getElementById('libType').value;
    // Collect folders
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
    const create_previews = document.querySelector('input[name="createPreviews"]:checked')?.value === 'yes';
    const create_thumbnails = document.querySelector('input[name="createThumbnails"]:checked')?.value === 'yes';
    const audio_normalization = document.querySelector('input[name="audioNormalization"]:checked')?.value === 'yes';
    const scan_interval = document.getElementById('editScanInterval')?.value || 'disabled';
    const watch_enabled = document.querySelector('input[name="watchEnabled"]:checked')?.value === 'yes';

    let adult_content_type = null;
    if (media_type === 'adult_movies') {
        adult_content_type = document.querySelector('input[name="adultContentType"]:checked')?.value || 'movies';
    }

    const d = await api('POST', '/libraries', {
        name, media_type, path: folders[0], folders, is_enabled: true,
        season_grouping, access_level, allowed_users,
        include_in_homepage, include_in_search, retrieve_metadata,
        nfo_import, nfo_export, prefer_local_artwork,
        create_previews, create_thumbnails, audio_normalization, adult_content_type,
        scan_interval, watch_enabled
    });
    if (d.success) { toast('Library created!'); loadLibrariesView(); loadSidebarCounts(); }
    else toast('Failed: ' + d.error, 'error');
}

// ──── Collections ────
