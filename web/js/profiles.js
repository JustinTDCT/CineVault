const AVATAR_PRESETS = [
    { id: 'av-lion',    emoji: '🦁', bg: ['#c0392b','#e74c3c'] },
    { id: 'av-fox',     emoji: '🦊', bg: ['#d35400','#e67e22'] },
    { id: 'av-panda',   emoji: '🐼', bg: ['#2c3e50','#34495e'] },
    { id: 'av-unicorn', emoji: '🦄', bg: ['#8e44ad','#9b59b6'] },
    { id: 'av-owl',     emoji: '🦉', bg: ['#795548','#8d6e63'] },
    { id: 'av-rocket',  emoji: '🚀', bg: ['#1565c0','#1e88e5'] },
    { id: 'av-star',    emoji: '⭐', bg: ['#f9a825','#fdd835'] },
    { id: 'av-alien',   emoji: '👽', bg: ['#2e7d32','#43a047'] },
    { id: 'av-robot',   emoji: '🤖', bg: ['#546e7a','#78909c'] },
    { id: 'av-ghost',   emoji: '👻', bg: ['#5c6bc0','#7986cb'] },
    { id: 'av-ninja',   emoji: '🥷', bg: ['#1a1a2e','#16213e'] },
    { id: 'av-dragon',  emoji: '🐉', bg: ['#b71c1c','#c62828'] },
    { id: 'av-cat',     emoji: '🐱', bg: ['#ff6f00','#ff8f00'] },
    { id: 'av-dog',     emoji: '🐶', bg: ['#6d4c41','#8d6e63'] },
    { id: 'av-bear',    emoji: '🐻', bg: ['#4e342e','#6d4c41'] },
    { id: 'av-penguin', emoji: '🐧', bg: ['#263238','#37474f'] },
];

function getAvatarById(avatarId) {
    return AVATAR_PRESETS.find(a => a.id === avatarId);
}

function renderAvatarCircle(avatarId, name, size) {
    const sz = size || 40;
    const av = getAvatarById(avatarId);
    if (av) {
        return `<div style="width:${sz}px;height:${sz}px;border-radius:50%;background:linear-gradient(135deg,${av.bg[0]},${av.bg[1]});display:flex;align-items:center;justify-content:center;font-size:${sz*0.5}px;">${av.emoji}</div>`;
    }
    // Fallback: gradient with initial
    const [bg, circle] = userColor(name || '?');
    const initial = (name || '?')[0].toUpperCase();
    return `<div style="width:${sz}px;height:${sz}px;border-radius:50%;background:radial-gradient(circle,${circle},${bg});display:flex;align-items:center;justify-content:center;font-size:${sz*0.45}px;font-weight:600;color:rgba(255,255,255,0.85);">${initial}</div>`;
}

// ──────────────────── Profile Switching (Household) ────────────────────

async function openProfileSwitch() {
    closeUserDropdown();
    const overlay = document.getElementById('profileSwitchOverlay');
    const grid = document.getElementById('profileSwitchGrid');
    overlay.classList.add('active');

    try {
        const data = await api('GET', '/household/profiles');
        const profiles = (data.success && data.data) ? data.data : [];

        grid.innerHTML = profiles.map(p => {
            const dn = p.display_name || p.username;
            const avatarHtml = renderAvatarCircle(p.avatar_id, dn, 80);
            const isActive = currentUser && currentUser.id === p.id;
            let badges = '';
            if (p.is_kids_profile) badges += '<div class="profile-switch-badge kids">Kids</div>';
            if (p.has_pin) badges += '<div class="profile-switch-badge" style="background:rgba(0,217,255,0.1);color:#00D9FF;border:1px solid rgba(0,217,255,0.2);">&#128274;</div>';
            return `<div class="profile-switch-user ${p.is_kids_profile ? 'kids-profile' : ''} ${isActive ? 'active' : ''}"
                onclick="switchToProfile('${p.id}', ${p.has_pin})"
                style="${isActive ? 'border-color:rgba(0,217,255,0.5);box-shadow:0 0 20px rgba(0,217,255,0.2);' : ''}">
                <div class="profile-switch-avatar">${avatarHtml}</div>
                <div class="profile-switch-name">${dn}</div>
                ${badges}
            </div>`;
        }).join('');
    } catch {
        grid.innerHTML = '<div style="color:#5a6a7f;">Could not load profiles</div>';
    }
}

function closeProfileSwitch() {
    document.getElementById('profileSwitchOverlay').classList.remove('active');
}

async function switchToProfile(profileId, hasPin) {
    // Always require PIN if profile has one, even if re-selecting current profile
    if (hasPin) {
        showInlinePinEntry(profileId);
        return;
    }

    // Same profile, no PIN — just mark picked and proceed
    if (currentUser && currentUser.id === profileId) {
        sessionStorage.setItem('profile_picked', '1');
        closeProfileSwitch();
        closeHouseholdPicker();
        loadHomeView();
        return;
    }

    // Different profile, no PIN — switch immediately
    try {
        const data = await api('POST', '/household/switch', { profile_id: profileId });
        if (data.success) {
            localStorage.setItem('token', data.data.token);
            localStorage.setItem('user', JSON.stringify(data.data.user));
            currentUser = data.data.user;
            sessionStorage.setItem('profile_picked', '1');
            closeProfileSwitch();
            closeHouseholdPicker();
            document.getElementById('userAvatar').textContent = currentUser.username[0].toUpperCase();
            updateTopBarAvatar();
            loadHomeView();
            loadSidebarCounts();
        } else {
            toast(data.error || 'Switch failed', 'error');
        }
    } catch {
        toast('Connection error', 'error');
    }
}

// Inline PIN entry for profile switch (used in both overlays)
let inlinePinTarget = null;
function showInlinePinEntry(profileId) {
    inlinePinTarget = profileId;
    // Determine which overlay is active
    const hpOverlay = document.getElementById('householdPickerOverlay');
    const psOverlay = document.getElementById('profileSwitchOverlay');
    const isHousehold = hpOverlay.classList.contains('active');
    const container = isHousehold ? hpOverlay : psOverlay;

    let pinDiv = container.querySelector('.hp-pin-overlay');
    if (!pinDiv) {
        pinDiv = document.createElement('div');
        pinDiv.className = 'hp-pin-overlay';
        container.appendChild(pinDiv);
    }

    const pLen = pinLength || 4;
    let boxesHtml = '';
    for (let i = 0; i < pLen; i++) boxesHtml += `<div class="hp-pin-box${i===0?' active':''}" id="hpBox${i}"></div>`;

    pinDiv.innerHTML = `
        <div class="hp-pin-title">Enter PIN</div>
        <div class="hp-pin-boxes">${boxesHtml}</div>
        <div class="hp-pin-error" id="hpPinError"></div>
        <div class="hp-pin-cancel" onclick="hideInlinePinEntry()">Cancel</div>
    `;
    pinDiv.style.display = 'flex';

    // Click anywhere on the PIN overlay to re-focus the hidden input
    pinDiv.onclick = function(e) {
        if (!e.target.classList.contains('hp-pin-cancel')) {
            document.getElementById('hpPinHidden').focus();
        }
    };

    const hi = document.getElementById('hpPinHidden');
    hi.value = '';
    hi.maxLength = pLen;
    setTimeout(() => hi.focus(), 100);
}

function hideInlinePinEntry() {
    inlinePinTarget = null;
    document.querySelectorAll('.hp-pin-overlay').forEach(el => el.style.display = 'none');
}

document.getElementById('hpPinHidden').addEventListener('input', async function() {
    const pLen = pinLength || 4;
    const val = this.value.replace(/\D/g, '').substring(0, pLen);
    this.value = val;
    for (let i = 0; i < pLen; i++) {
        const box = document.getElementById('hpBox' + i);
        if (!box) continue;
        box.textContent = i < val.length ? '\u25CF' : '';
        box.className = 'hp-pin-box' + (i < val.length ? ' filled' : '') + (i === val.length ? ' active' : '');
    }
    if (val.length === pLen && inlinePinTarget) {
        const errEl = document.getElementById('hpPinError');
        try {
            const data = await api('POST', '/household/switch', { profile_id: inlinePinTarget, pin: val });
            if (data.success) {
                localStorage.setItem('token', data.data.token);
                localStorage.setItem('user', JSON.stringify(data.data.user));
                currentUser = data.data.user;
                sessionStorage.setItem('profile_picked', '1');
                hideInlinePinEntry();
                closeProfileSwitch();
                closeHouseholdPicker();
                document.getElementById('userAvatar').textContent = currentUser.username[0].toUpperCase();
                updateTopBarAvatar();
                loadHomeView();
                loadSidebarCounts();
            } else {
                if (errEl) errEl.textContent = data.error || 'Invalid PIN';
                this.value = '';
                for (let i = 0; i < pLen; i++) {
                    const box = document.getElementById('hpBox' + i);
                    if (box) { box.textContent = ''; box.className = 'hp-pin-box' + (i===0?' active':''); }
                }
            }
        } catch {
            if (errEl) errEl.textContent = 'Connection error';
            this.value = '';
        }
    }
});

document.getElementById('hpPinHidden').addEventListener('keydown', function(e) {
    if (e.key === 'Escape') hideInlinePinEntry();
});

// ──────────────────── Household "Who's Watching?" Picker ────────────────────

async function showHouseholdPicker() {
    try {
        const data = await api('GET', '/household/profiles');
        const profiles = (data.success && data.data) ? data.data : [];

        // If only one profile (master with no sub-profiles), skip picker
        if (profiles.length <= 1) {
            sessionStorage.setItem('profile_picked', '1');
            loadHomeView();
            return;
        }

        const overlay = document.getElementById('householdPickerOverlay');
        const grid = document.getElementById('householdPickerGrid');
        overlay.classList.add('active');

        grid.innerHTML = profiles.map(p => {
            const dn = p.display_name || p.username;
            const avatarHtml = renderAvatarCircle(p.avatar_id, dn, 72);
            let badges = '';
            if (p.is_kids_profile) badges += '<div class="household-picker-badge kids">Kids</div>';
            if (p.has_pin) badges += '<div class="household-picker-badge pin">&#128274;</div>';
            return `<div class="household-picker-card ${p.is_kids_profile ? 'kids-profile' : ''}"
                onclick="switchToProfile('${p.id}', ${p.has_pin})">
                <div class="household-picker-avatar">${avatarHtml}</div>
                <div class="household-picker-name">${dn}</div>
                ${badges}
            </div>`;
        }).join('');
    } catch {
        // On error, skip picker and go to home
        sessionStorage.setItem('profile_picked', '1');
        loadHomeView();
    }
}

function closeHouseholdPicker() {
    document.getElementById('householdPickerOverlay').classList.remove('active');
}

// ──────────────────── Manage Profiles ────────────────────

let mpProfiles = [];
let mpEditingId = null;

async function openManageProfiles() {
    closeUserDropdown();
    const overlay = document.getElementById('manageProfilesOverlay');
    overlay.classList.add('active');
    await mpLoadProfiles();
}

function closeManageProfiles() {
    document.getElementById('manageProfilesOverlay').classList.remove('active');
    document.getElementById('mpFormArea').innerHTML = '';
    mpEditingId = null;
}

async function mpLoadProfiles() {
    try {
        const data = await api('GET', '/household/profiles');
        mpProfiles = (data.success && data.data) ? data.data : [];
    } catch { mpProfiles = []; }

    const list = document.getElementById('mpProfileList');
    const subProfiles = mpProfiles.filter(p => !p.is_master);
    const addBtn = document.getElementById('mpAddBtn');

    list.innerHTML = subProfiles.map(p => {
        const dn = p.display_name || p.username;
        const avatarHtml = renderAvatarCircle(p.avatar_id, dn, 44);
        let meta = [];
        if (p.is_kids_profile) meta.push('Kids');
        if (p.max_content_rating) meta.push('Max: ' + p.max_content_rating);
        if (p.has_pin) meta.push('PIN set');
        return `<div class="mp-profile-item">
            <div class="mp-profile-avatar">${avatarHtml}</div>
            <div class="mp-profile-info">
                <div class="mp-profile-name">${dn}</div>
                <div class="mp-profile-meta">${meta.join(' · ') || 'No restrictions'}</div>
            </div>
            <div class="mp-profile-actions">
                <button class="mp-btn-edit" onclick="mpShowEditForm('${p.id}')">Edit</button>
                <button class="mp-btn-delete" onclick="mpDeleteProfile('${p.id}', '${dn}')">Delete</button>
            </div>
        </div>`;
    }).join('');

    if (subProfiles.length === 0) {
        list.innerHTML = '<div style="color:#5a6a7f;text-align:center;padding:16px;">No sub-profiles yet</div>';
    }

    addBtn.disabled = subProfiles.length >= 5;
    addBtn.textContent = subProfiles.length >= 5 ? 'Maximum 5 profiles reached' : '+ Add Profile';
}

function mpRenderForm(title, profile) {
    const isEdit = !!profile;
    const selAvatar = (profile && profile.avatar_id) || '';
    const selKids = profile ? profile.is_kids_profile : false;
    const selRating = (profile && profile.max_content_rating) || '';
    const selName = (profile && profile.display_name) || '';

    const avatarOptions = AVATAR_PRESETS.map(a => {
        const selected = a.id === selAvatar ? ' selected' : '';
        return `<div class="mp-avatar-option${selected}" 
            style="background:linear-gradient(135deg,${a.bg[0]},${a.bg[1]})"
            onclick="mpSelectAvatar(this, '${a.id}')" data-avatar="${a.id}">
            ${a.emoji}
        </div>`;
    }).join('');

    return `<div class="mp-form">
        <div class="mp-form-title">${title}</div>
        <div class="form-group">
            <label>Display Name</label>
            <input type="text" id="mpFormName" value="${selName}" placeholder="e.g. Kids, Guest">
        </div>
        <div class="form-group">
            <label>Avatar</label>
            <div class="mp-avatar-grid">${avatarOptions}</div>
            <input type="hidden" id="mpFormAvatar" value="${selAvatar}">
        </div>
        <div class="mp-form-row">
            <div class="form-group">
                <label>Rating Country</label>
                <select id="mpFormRatingCountry" onchange="updateFormRatingOptions()">
                    <option value="US" selected>US (MPAA)</option>
                    <option value="GB">UK (BBFC)</option>
                    <option value="CA">Canada</option>
                    <option value="AU">Australia</option>
                    <option value="DE">Germany</option>
                    <option value="FR">France</option>
                    <option value="JP">Japan</option>
                </select>
            </div>
            <div class="form-group">
                <label>Max Content Rating</label>
                <select id="mpFormRating">
                    <option value="" ${selRating===''?'selected':''}>Unrestricted</option>
                    <option value="G" ${selRating==='G'?'selected':''}>G</option>
                    <option value="PG" ${selRating==='PG'?'selected':''}>PG</option>
                    <option value="PG-13" ${selRating==='PG-13'?'selected':''}>PG-13</option>
                    <option value="R" ${selRating==='R'?'selected':''}>R</option>
                    <option value="NC-17" ${selRating==='NC-17'?'selected':''}>NC-17</option>
                </select>
            </div>
            <div class="form-group">
                <label>PIN (optional)</label>
                <input type="text" id="mpFormPin" inputmode="numeric" maxlength="10" placeholder="${isEdit ? 'Leave blank to keep' : '4-digit PIN'}">
            </div>
        </div>
        <div class="form-group">
            <div class="mp-kids-toggle">
                <input type="checkbox" id="mpFormKids" ${selKids?'checked':''}>
                <label for="mpFormKids" style="margin:0;color:#e5e5e5;font-size:0.85rem;">Kids Profile</label>
            </div>
        </div>
        <div class="mp-form-actions">
            <button class="mp-form-btn save" onclick="mpSaveForm()">${isEdit ? 'Save Changes' : 'Create Profile'}</button>
            <button class="mp-form-btn cancel" onclick="mpCancelForm()">Cancel</button>
        </div>
    </div>`;
}

function mpSelectAvatar(el, avatarId) {
    el.closest('.mp-avatar-grid').querySelectorAll('.mp-avatar-option').forEach(o => o.classList.remove('selected'));
    el.classList.add('selected');
    document.getElementById('mpFormAvatar').value = avatarId;
}

function mpShowAddForm() {
    mpEditingId = null;
    document.getElementById('mpFormArea').innerHTML = mpRenderForm('Add Profile', null);
}

function mpShowEditForm(profileId) {
    const p = mpProfiles.find(x => x.id === profileId);
    if (!p) return;
    mpEditingId = profileId;
    document.getElementById('mpFormArea').innerHTML = mpRenderForm('Edit Profile', p);
}

function mpCancelForm() {
    mpEditingId = null;
    document.getElementById('mpFormArea').innerHTML = '';
}

async function mpSaveForm() {
    const name = document.getElementById('mpFormName').value.trim();
    const avatar = document.getElementById('mpFormAvatar').value || null;
    const rating = document.getElementById('mpFormRating').value || null;
    const pin = document.getElementById('mpFormPin').value.trim();
    const kids = document.getElementById('mpFormKids').checked;

    if (!name) { toast('Display name is required', 'error'); return; }

    if (mpEditingId) {
        // Update existing
        const body = {
            display_name: name,
            avatar_id: avatar,
            max_content_rating: rating,
            is_kids_profile: kids,
        };
        if (pin !== '') body.pin = pin;
        const data = await api('PUT', '/household/profiles/' + mpEditingId, body);
        if (data.success) {
            toast('Profile updated');
            mpCancelForm();
            await mpLoadProfiles();
        } else {
            toast(data.error || 'Update failed', 'error');
        }
    } else {
        // Create new
        const body = {
            display_name: name,
            avatar_id: avatar,
            max_content_rating: rating,
            is_kids_profile: kids,
        };
        if (pin !== '') body.pin = pin;
        const data = await api('POST', '/household/profiles', body);
        if (data.success) {
            toast('Profile created');
            mpCancelForm();
            await mpLoadProfiles();
        } else {
            toast(data.error || 'Create failed', 'error');
        }
    }
}

async function mpDeleteProfile(profileId, name) {
    if (!confirm('Delete profile "' + name + '"? This cannot be undone.')) return;
    const data = await api('DELETE', '/household/profiles/' + profileId);
    if (data.success) {
        toast('Profile deleted');
        await mpLoadProfiles();
    } else {
        toast(data.error || 'Delete failed', 'error');
    }
}

// ──────────────────── Kids Mode Helpers ────────────────────
function isKidsMode() {
    return currentUser && currentUser.is_kids_profile === true;
}

function getUserMaxRating() {
    return (currentUser && currentUser.max_content_rating) || null;
}

// ──────────────────── Updated Home View with Recommendations ────────────────────
// Override loadHomeView to add recommendations and kids banner
const _origLoadHomeView = loadHomeView;
loadHomeView = async function() {
    const mc = document.getElementById('mainContent');
    const kidsMode = isKidsMode();

    let kidsBannerHtml = '';
    if (kidsMode) {
        kidsBannerHtml = `
            <div class="kids-banner">
                <div class="kids-banner-icon">🧒</div>
                <div>
                    <div class="kids-banner-text">Kids Mode Active</div>
                    <div class="kids-banner-sub">Content is filtered to ${getUserMaxRating() || 'PG'}-rated and below</div>
                </div>
            </div>`;
    }

    mc.innerHTML = `
        ${kidsBannerHtml}
        <div class="section-header"><h2 class="section-title">Continue Watching</h2></div>
        <div id="continueRow" class="continue-row"></div>
        <div class="section-header"><h2 class="section-title">Recommended For You</h2></div>
        <div id="recoRow" class="reco-row"><div class="spinner"></div></div>
        <div id="bywRows"></div>
        <div class="section-header"><h2 class="section-title">Recently Added</h2></div>
        <div class="media-grid" id="recentGrid"></div>`;

    // Continue Watching
    try {
        const cw = await api('GET', '/watch/continue');
        const row = document.getElementById('continueRow');
        if (cw.success && cw.data && cw.data.length > 0) {
            row.innerHTML = cw.data.map(wh => {
                const item = wh.media_item || {};
                const pct = wh.duration_seconds ? Math.round(wh.progress_seconds/wh.duration_seconds*100) : 0;
                return `<div class="media-card" onclick="playMedia('${item.id}','${item.title||''}')">
                    <div class="media-poster">
                        ${item.poster_path ? '<img src="'+posterSrc(item.poster_path, item.updated_at)+'">' : mediaIcon(item.media_type||'movies')}
                        <div class="progress-bar"><div class="progress-fill" style="width:${pct}%"></div></div>
                        <div class="play-overlay"><div class="play-button">&#9654;</div></div>
                    </div>
                    <div class="media-info"><div class="media-title">${item.title||'Unknown'}</div><div class="media-meta">${Math.floor(wh.progress_seconds/60)}/${wh.duration_seconds?Math.floor(wh.duration_seconds/60):'?'} min</div></div>
                </div>`;
            }).join('');
        } else row.innerHTML = '<div style="color:#5a6a7f;padding:20px;">No items in progress</div>';
    } catch { document.getElementById('continueRow').innerHTML = ''; }

    // Recommendations
    try {
        const reco = await api('GET', '/recommendations');
        const recoRow = document.getElementById('recoRow');
        if (reco.success && reco.data && reco.data.length > 0) {
            recoRow.innerHTML = reco.data.map(item => renderMediaCard(item)).join('');
        } else {
            recoRow.innerHTML = '<div style="color:#5a6a7f;padding:20px;">Watch more to get personalized recommendations</div>';
        }
    } catch { document.getElementById('recoRow').innerHTML = '<div style="color:#5a6a7f;padding:20px;">Recommendations unavailable</div>'; }

    // Because You Watched
    try {
        const byw = await api('GET', '/recommendations/because-you-watched');
        const bywContainer = document.getElementById('bywRows');
        if (byw.success && byw.data && byw.data.length > 0) {
            bywContainer.innerHTML = byw.data.map(row => {
                const srcTitle = row.source_item ? row.source_item.title : 'Unknown';
                const items = row.similar_items || [];
                return `
                    <div class="section-header"><h2 class="section-title">Because You Watched <em>${srcTitle}</em></h2></div>
                    <div class="reco-row">${items.map(item => renderMediaCard(item)).join('')}</div>`;
            }).join('');
        }
    } catch { /* silent fail for BYW */ }

    // Recently Added
    try {
        const libs = await api('GET', '/libraries');
        const grid = document.getElementById('recentGrid');
        if (libs.success && libs.data) {
            const homepageLibs = libs.data.filter(lib => lib.include_in_homepage !== false);
            let allItems = [];
            for (const lib of homepageLibs.slice(0,5)) {
                const m = await api('GET', '/libraries/'+lib.id+'/media');
                if (m.success && m.data && m.data.items) allItems = allItems.concat(m.data.items);
            }
            allItems.sort((a,b) => new Date(b.added_at) - new Date(a.added_at));
            grid.innerHTML = allItems.length > 0
                ? allItems.slice(0,12).map(renderMediaCard).join('')
                : `<div class="empty-state" style="grid-column:1/-1;"><div class="empty-state-icon">&#128253;</div><div class="empty-state-title">No media yet</div><p>Add libraries and scan them to populate your media</p><button class="btn-primary" style="margin-top:18px;" onclick="navigate('libraries')">Manage Libraries</button></div>`;
        }
    } catch {}
};

// ──────────────────── Updated Profile View ────────────────────
const _origLoadProfileView = loadProfileView;
loadProfileView = async function() {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Edit Profile</h2></div><div class="settings-grid" id="profileGrid"><div class="spinner"></div></div>`;

    const profileData = await api('GET', '/profile');
    if (!profileData.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Failed to load profile</div></div>'; return; }
    const u = profileData.data;

    // Get profile settings
    const settingsData = await api('GET', '/profile/settings');
    const settings = settingsData.success ? settingsData.data : {};

    // Get PIN length setting for validation
    let profPinLength = 4;
    try {
        const flSettings = await api('GET', '/auth/fast-login/settings');
        if (flSettings.success && flSettings.data) profPinLength = parseInt(flSettings.data.fast_login_pin_length) || 4;
    } catch(e) {}

    const currentRating = settings.max_content_rating || '';
    const isKids = settings.is_kids_profile || false;
    const currentAvatar = settings.avatar_id || '';

    // Build avatar grid HTML
    const avatarGridHtml = AVATAR_PRESETS.map(av => {
        const sel = (currentAvatar === av.id) ? 'selected' : '';
        return `<div class="avatar-option ${sel}" data-avatar="${av.id}" onclick="selectAvatar('${av.id}')"
            style="background:linear-gradient(135deg,${av.bg[0]},${av.bg[1]});">${av.emoji}</div>`;
    }).join('');

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
            <h3>Profile Avatar</h3>
            <p style="color:#8a9bae;font-size:0.82rem;margin-bottom:12px;">Choose an avatar for your profile</p>
            <div class="avatar-grid" id="avatarGrid">${avatarGridHtml}</div>
            <input type="hidden" id="profAvatar" value="${currentAvatar}">
            <button class="btn-primary" onclick="saveProfileSettings()" style="margin-top:8px;">Save Avatar</button>
        </div>
        <div class="settings-card">
            <h3>Parental Controls</h3>
            <p style="color:#8a9bae;font-size:0.82rem;margin-bottom:12px;">Set a maximum content rating for this profile. Content above this rating will be hidden. Select a country to use that country's rating system.</p>
            <div class="form-group">
                <label>Rating Country</label>
                <select class="rating-select" id="profRatingCountry" onchange="updateRatingOptions()">
                    <option value="US" selected>United States (MPAA)</option>
                    <option value="GB">United Kingdom (BBFC)</option>
                    <option value="CA">Canada</option>
                    <option value="AU">Australia</option>
                    <option value="DE">Germany (FSK)</option>
                    <option value="FR">France</option>
                    <option value="JP">Japan</option>
                </select>
            </div>
            <div class="form-group">
                <label>Maximum Content Rating</label>
                <select class="rating-select" id="profMaxRating">
                    <option value="" ${!currentRating ? 'selected' : ''}>No Restriction</option>
                    <option value="G" ${currentRating === 'G' ? 'selected' : ''}>G — General Audiences</option>
                    <option value="PG" ${currentRating === 'PG' ? 'selected' : ''}>PG — Parental Guidance</option>
                    <option value="PG-13" ${currentRating === 'PG-13' ? 'selected' : ''}>PG-13 — Parents Strongly Cautioned</option>
                    <option value="R" ${currentRating === 'R' ? 'selected' : ''}>R — Restricted</option>
                    <option value="NC-17" ${currentRating === 'NC-17' ? 'selected' : ''}>NC-17 — Adults Only</option>
                </select>
            </div>
            <div class="kids-toggle-container">
                <label class="toggle-switch">
                    <input type="checkbox" id="profKidsMode" ${isKids ? 'checked' : ''}>
                    <span class="toggle-slider"></span>
                </label>
                <div>
                    <div style="color:#e5e5e5;font-size:0.85rem;font-weight:600;">Kids Profile</div>
                    <div style="color:#5a6a7f;font-size:0.78rem;">Enables simplified UI and auto-sets rating to PG when turned on</div>
                </div>
            </div>
            <button class="btn-primary" onclick="saveProfileSettings()" style="margin-top:8px;">Save Parental Settings</button>
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
                <p style="color:#8a9bae;font-size:0.82rem;margin-bottom:12px;">Set a ${profPinLength}-digit PIN for quick login. ${u.has_pin ? '<span style="color:#51cf66;">PIN is set.</span>' : '<span style="color:#5a6a7f;">No PIN set.</span>'}</p>
                <div class="form-group">
                    <label>New PIN (${profPinLength} digits)</label>
                    <input type="password" id="profPin" placeholder="Enter ${profPinLength}-digit PIN" maxlength="${profPinLength}" pattern="[0-9]*" inputmode="numeric">
                </div>
                <button class="btn-primary" onclick="saveProfilePin(${profPinLength})">Set PIN</button>
            </div>
        </div>
        <div class="settings-card full-width">
            <h3>Overlay Badges</h3>
            <p style="color:#8a9bae;font-size:0.82rem;margin-bottom:16px;">Choose which badges appear on poster cards. Syncs across all your devices.</p>
            <div id="profileOverlayToggles"><div class="spinner"></div></div>
        </div>
        <div class="settings-card">
            <h3>Two-Factor Authentication</h3>
            <p style="color:#8a9bae;font-size:0.82rem;margin-bottom:12px;">Add an extra layer of security with a TOTP authenticator app.</p>
            <div id="prof2FAArea"><div class="spinner"></div></div>
        </div>
        <div class="settings-card">
            <h3>Connected Services</h3>
            <p style="color:#8a9bae;font-size:0.82rem;margin-bottom:16px;">Link external accounts to sync your activity.</p>
            <div class="connected-service-block" id="profTraktArea"><div class="spinner"></div></div>
            <div class="connected-service-block" id="profLastfmArea" style="margin-top:16px;padding-top:16px;border-top:1px solid rgba(0,217,255,0.1);"><div class="spinner"></div></div>
        </div>
        <div class="settings-card">
            <h3>Home Layout</h3>
            <p style="color:#8a9bae;font-size:0.82rem;margin-bottom:12px;">Choose which sections appear on your home screen and their order.</p>
            <div id="profHomeLayout"><div class="spinner"></div></div>
        </div>
        <div class="settings-card">
            <h3>Playback Preferences</h3>
            <p style="color:#8a9bae;font-size:0.82rem;margin-bottom:12px;">Set your default streaming quality and playback behavior.</p>
            <div id="profPlaybackPrefs"><div class="spinner"></div></div>
        </div>`;

    // Load all profile sub-sections
    loadProfileOverlayToggles();
    loadProfile2FA();
    loadProfileTrakt();
    loadProfileLastfm();
    loadProfileHomeLayout();
    loadProfilePlaybackPrefs();
};

const OVERLAY_POSITIONS = [
    { id: 'top-left', label: 'Top Left' }, { id: 'top', label: 'Top' }, { id: 'top-right', label: 'Top Right' },
    { id: 'left', label: 'Left Side' }, { id: 'right', label: 'Right Side' },
    { id: 'bottom-left', label: 'Bottom Left' }, { id: 'bottom', label: 'Bottom' }, { id: 'bottom-right', label: 'Bottom Right' }
];
const OVERLAY_ZONES = {
    'top-left': 'top', 'top': 'top', 'top-right': 'top',
    'bottom-left': 'bottom', 'bottom': 'bottom', 'bottom-right': 'bottom',
    'left': 'left', 'right': 'right'
};
const OVERLAY_GROUPS = [
    { key: 'resolution_audio', label: 'Resolution & Audio' },
    { key: 'edition', label: 'Edition' },
    { key: 'ratings', label: 'Ratings' },
    { key: 'content_rating', label: 'Content Rating' },
    { key: 'source_type', label: 'Source Type' }
];

function _overlayPositionsTaken(groups, excludeKey) {
    const takenZones = new Set();
    for (const g of OVERLAY_GROUPS) {
        if (g.key === excludeKey) continue;
        const gd = groups[g.key];
        if (gd && gd.enabled && gd.position) {
            takenZones.add(OVERLAY_ZONES[gd.position]);
        }
    }
    return takenZones;
}

function _renderPositionDropdown(groupKey, selectedPos, groups) {
    const takenZones = _overlayPositionsTaken(groups, groupKey);
    let opts = '';
    for (const p of OVERLAY_POSITIONS) {
        const zone = OVERLAY_ZONES[p.id];
        const disabled = takenZones.has(zone) ? 'disabled' : '';
        const selected = p.id === selectedPos ? 'selected' : '';
        opts += `<option value="${p.id}" ${selected} ${disabled}>${p.label}</option>`;
    }
    return `<select id="profOvPos_${groupKey}" class="ov-pos-select" onchange="overlayPosChanged()">${opts}</select>`;
}

function _renderOverlayGroupRow(group, data, groups) {
    const enabled = data && data.enabled;
    const pos = (data && data.position) || 'top-right';
    return `<div class="overlay-group-row" style="display:flex;align-items:center;gap:12px;padding:10px 0;border-bottom:1px solid rgba(255,255,255,0.06);">
        <label class="toggle-label" style="margin:0;display:flex;align-items:center;gap:8px;flex:0 0 auto;">
            <span class="toggle-switch"><input type="checkbox" id="profOvEn_${group.key}" ${enabled?'checked':''} onchange="overlayPosChanged()"><span class="toggle-slider"></span></span>
        </label>
        <span style="flex:1;font-weight:600;font-size:0.88rem;">${group.label}</span>
        ${_renderPositionDropdown(group.key, pos, groups)}
    </div>`;
}

function overlayPosChanged() {
    const groups = {};
    for (const g of OVERLAY_GROUPS) {
        groups[g.key] = {
            enabled: document.getElementById('profOvEn_' + g.key).checked,
            position: document.getElementById('profOvPos_' + g.key).value
        };
    }
    // Re-render dropdowns to update disabled states
    for (const g of OVERLAY_GROUPS) {
        const sel = document.getElementById('profOvPos_' + g.key);
        const curVal = sel.value;
        const takenZones = _overlayPositionsTaken(groups, g.key);
        for (const opt of sel.options) {
            const zone = OVERLAY_ZONES[opt.value];
            opt.disabled = takenZones.has(zone);
        }
    }
}

async function loadProfileOverlayToggles() {
    const res = await api('GET', '/settings/display');
    const raw = res.success && res.data ? res.data.overlay_settings : null;
    const p = migrateOverlayPrefs(raw);

    const groups = p.groups || {
        resolution_audio: { enabled: true, position: 'top-right' },
        edition: { enabled: true, position: 'top-left' },
        ratings: { enabled: true, position: 'bottom-left' },
        content_rating: { enabled: false, position: 'bottom-right' },
        source_type: { enabled: false, position: 'bottom-right' }
    };

    let rowsHTML = '';
    for (const g of OVERLAY_GROUPS) {
        rowsHTML += _renderOverlayGroupRow(g, groups[g.key], groups);
    }

    document.getElementById('profileOverlayToggles').innerHTML = `
        <div class="overlay-groups-container">
            ${rowsHTML}
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px 20px;margin-top:14px;">
                <label class="toggle-label" style="margin:0;display:flex;align-items:center;gap:8px;font-size:0.82rem;">
                    <span class="toggle-switch"><input type="checkbox" id="profOvHideTheatrical" ${p.hide_theatrical !== false?'checked':''}><span class="toggle-slider"></span></span>
                    Hide Theatrical Overlay
                </label>
            </div>
        </div>
        <button class="btn-primary" onclick="saveProfileOverlayPrefs()" style="margin-top:16px;">Save Overlay Settings</button>`;
}

async function saveProfileOverlayPrefs() {
    const groups = {};
    for (const g of OVERLAY_GROUPS) {
        groups[g.key] = {
            enabled: document.getElementById('profOvEn_' + g.key).checked,
            position: document.getElementById('profOvPos_' + g.key).value
        };
    }
    const settings = {
        groups,
        resolution_hdr: groups.resolution_audio.enabled,
        audio_codec: groups.resolution_audio.enabled,
        ratings: groups.ratings.enabled,
        content_rating: groups.content_rating.enabled,
        edition_type: groups.edition.enabled,
        source_type: groups.source_type.enabled,
        hide_theatrical: document.getElementById('profOvHideTheatrical').checked,
    };
    const d = await api('PUT', '/settings/display', { overlay_settings: settings });
    if (d.success) {
        overlayPrefs = settings;
        toast('Overlay settings saved!');
    } else toast(d.error || 'Failed to save', 'error');
}

function selectAvatar(avatarId) {
    document.getElementById('profAvatar').value = avatarId;
    document.querySelectorAll('.avatar-option').forEach(el => {
        el.classList.toggle('selected', el.dataset.avatar === avatarId);
    });
}

async function saveProfileSettings() {
    const maxRating = document.getElementById('profMaxRating').value;
    const isKids = document.getElementById('profKidsMode').checked;
    const avatarId = document.getElementById('profAvatar').value;

    const body = {
        max_content_rating: maxRating || '',
        is_kids_profile: isKids,
        avatar_id: avatarId || null
    };

    const d = await api('PUT', '/profile/settings', body);
    if (d.success) {
        toast('Profile settings updated!');
        // Update local user data and token
        if (d.data.user) {
            currentUser = { ...currentUser, ...d.data.user };
            localStorage.setItem('user', JSON.stringify(currentUser));
        }
        if (d.data.token) {
            localStorage.setItem('token', d.data.token);
        }
        // Update avatar in top bar
        updateTopBarAvatar();
    } else toast(d.error || 'Failed to update settings', 'error');
}

// ──────────────────── Top Bar Avatar Update ────────────────────
function updateTopBarAvatar() {
    const avatarEl = document.getElementById('userAvatar');
    if (!currentUser) return;
    const av = currentUser.avatar_id ? getAvatarById(currentUser.avatar_id) : null;
    if (av) {
        avatarEl.innerHTML = av.emoji;
        avatarEl.style.background = `linear-gradient(135deg, ${av.bg[0]}, ${av.bg[1]})`;
        avatarEl.style.fontSize = '1.2rem';
    } else {
        avatarEl.textContent = currentUser.username[0].toUpperCase();
        avatarEl.style.background = 'linear-gradient(135deg, #00D9FF 0%, #0099CC 100%)';
        avatarEl.style.fontSize = '0.95rem';
    }
}

// Patch checkAuth to update avatar after login
const _origCheckAuth = checkAuth;
checkAuth = async function() {
    await _origCheckAuth();
    if (currentUser) updateTopBarAvatar();
};

// Update fast login to use avatars
const _origFastLoginShowUsers = fastLoginShowUsers;
fastLoginShowUsers = function() {
    const grid = document.getElementById('fastLoginGrid');
    const pinEntry = document.getElementById('pinEntryContainer');
    const back = document.getElementById('fastLoginBack');
    const title = document.getElementById('fastLoginTitle');
    const fallback = document.querySelector('.fast-login-fallback');
    grid.style.display = 'flex';
    pinEntry.classList.remove('active');
    back.style.display = 'none';
    title.textContent = 'Who\'s Watching?';
    if (fallback) fallback.style.display = '';
    selectedFastUser = null;

    grid.innerHTML = fastLoginUsers.map(u => {
        const dn = u.display_name || u.username;
        const avatarHtml = renderAvatarCircle(u.avatar_id, dn, 100);
        let badges = '';
        if (u.has_pin) badges += '<span class="fast-login-badge pin-set">&#128274;</span>';
        if (u.role === 'admin') badges += '<span class="fast-login-badge admin">&#128081;</span>';
        if (u.is_kids_profile) badges += '<span class="fast-login-badge" style="background:rgba(76,175,80,0.15);color:#4caf50;border:1px solid rgba(76,175,80,0.3);">Kids</span>';
        return `<div class="fast-login-user ${u.is_kids_profile ? 'kids-profile' : ''}" onclick="fastLoginSelectUser('${u.id}')">
            <div class="fast-login-avatar" style="background:none;">${avatarHtml}
            <div class="fast-login-badges" style="position:absolute;bottom:-4px;left:0;right:0;display:flex;justify-content:center;gap:4px;">${badges}</div></div>
            <div class="fast-login-user-name">${dn}</div></div>`;
    }).join('');
};

// ──── Sidebar Toggle (Mobile) ────
function toggleSidebar() {
    const sidebar = document.getElementById('sidebar');
    const overlay = document.getElementById('sidebarOverlay');
    sidebar.classList.toggle('open');
    overlay.classList.toggle('active');
}

// ──── B1: Two-Factor Authentication ────
async function loadProfile2FA() {
    const area = document.getElementById('prof2FAArea');
    if (!area) return;
    try {
        const res = await api('GET', '/auth/2fa/status');
        const enabled = res.success && res.data && res.data.enabled;
        if (enabled) {
            area.innerHTML = `
                <div style="display:flex;align-items:center;gap:12px;margin-bottom:12px;">
                    <span class="tag tag-green" style="font-size:0.85rem;">2FA Active</span>
                    <span style="color:#8a9bae;font-size:0.82rem;">Your account is protected with two-factor authentication.</span>
                </div>
                <button class="btn-danger btn-small" onclick="disable2FA()">Disable 2FA</button>`;
        } else {
            area.innerHTML = `
                <p style="color:#5a6a7f;font-size:0.82rem;margin-bottom:12px;">2FA is not enabled. Set it up with an authenticator app like Google Authenticator or Authy.</p>
                <button class="btn-primary" onclick="setup2FA()">Enable 2FA</button>`;
        }
    } catch {
        area.innerHTML = '<p style="color:#5a6a7f;">Could not load 2FA status</p>';
    }
}

async function setup2FA() {
    const area = document.getElementById('prof2FAArea');
    area.innerHTML = '<div class="spinner"></div> Setting up...';
    const res = await api('POST', '/auth/2fa/setup');
    if (!res.success) { toast(res.error || 'Failed to start 2FA setup', 'error'); loadProfile2FA(); return; }
    const data = res.data || {};
    area.innerHTML = `
        <div class="twofa-setup">
            <p style="color:#e5e5e5;margin-bottom:12px;">Scan this QR code with your authenticator app:</p>
            ${data.qr_code_url ? `<img src="${data.qr_code_url}" alt="2FA QR Code" class="twofa-qr">` :
              data.secret ? `<div class="twofa-secret"><code>${data.secret}</code></div><p style="color:#5a6a7f;font-size:0.78rem;">Enter this code manually in your authenticator app.</p>` : ''}
            <div class="form-group" style="margin-top:16px;">
                <label>Verification Code</label>
                <input type="text" id="twofa-verify-code" placeholder="Enter 6-digit code" maxlength="6" inputmode="numeric" style="max-width:200px;">
            </div>
            <div style="display:flex;gap:10px;">
                <button class="btn-primary" onclick="verify2FA()">Verify &amp; Enable</button>
                <button class="btn-secondary" onclick="loadProfile2FA()">Cancel</button>
            </div>
        </div>`;
}

async function verify2FA() {
    const code = document.getElementById('twofa-verify-code')?.value;
    if (!code || code.length < 6) { toast('Enter a 6-digit code', 'error'); return; }
    const res = await api('POST', '/auth/2fa/verify', { code });
    if (res.success) { toast('2FA enabled successfully!'); loadProfile2FA(); }
    else toast(res.error || 'Invalid code', 'error');
}

async function disable2FA() {
    if (!confirm('Disable two-factor authentication? Your account will be less secure.')) return;
    const res = await api('DELETE', '/auth/2fa');
    if (res.success) { toast('2FA disabled'); loadProfile2FA(); }
    else toast(res.error || 'Failed to disable 2FA', 'error');
}

// ──── B2: Trakt.tv Connection ────
let traktPollTimer = null;

async function loadProfileTrakt() {
    const area = document.getElementById('profTraktArea');
    if (!area) return;
    try {
        const res = await api('GET', '/trakt/status');
        const connected = res.success && res.data && res.data.connected;
        if (connected) {
            area.innerHTML = `
                <div style="display:flex;align-items:center;gap:12px;">
                    <strong style="color:#e5e5e5;">Trakt.tv</strong>
                    <span class="tag tag-green">Connected</span>
                    <span style="color:#8a9bae;font-size:0.82rem;">${res.data.username || ''}</span>
                    <button class="btn-danger btn-small" style="margin-left:auto;" onclick="disconnectTrakt()">Disconnect</button>
                </div>`;
        } else {
            area.innerHTML = `
                <div style="display:flex;align-items:center;gap:12px;">
                    <strong style="color:#e5e5e5;">Trakt.tv</strong>
                    <span class="tag tag-cyan" style="opacity:0.5;">Not Connected</span>
                    <button class="btn-primary btn-small" style="margin-left:auto;" onclick="connectTrakt()">Connect</button>
                </div>
                <p style="color:#5a6a7f;font-size:0.78rem;margin-top:6px;">Sync your watch history, ratings, and watchlist with Trakt.tv</p>`;
        }
    } catch {
        area.innerHTML = '<p style="color:#5a6a7f;">Could not load Trakt status</p>';
    }
}

async function connectTrakt() {
    const area = document.getElementById('profTraktArea');
    area.innerHTML = '<div class="spinner"></div> Requesting device code...';
    const res = await api('POST', '/trakt/device-code');
    if (!res.success) { toast(res.error || 'Failed to get device code', 'error'); loadProfileTrakt(); return; }
    const data = res.data || {};
    area.innerHTML = `
        <div class="trakt-auth-flow">
            <p style="color:#e5e5e5;margin-bottom:8px;"><strong>Step 1:</strong> Go to <a href="${data.verification_url || 'https://trakt.tv/activate'}" target="_blank" style="color:#00D9FF;">${data.verification_url || 'trakt.tv/activate'}</a></p>
            <p style="color:#e5e5e5;margin-bottom:12px;"><strong>Step 2:</strong> Enter this code:</p>
            <div class="trakt-device-code">${data.user_code || '----'}</div>
            <p style="color:#5a6a7f;font-size:0.78rem;margin-top:8px;">Waiting for authorization... This will update automatically.</p>
            <button class="btn-secondary btn-small" style="margin-top:8px;" onclick="cancelTraktAuth()">Cancel</button>
        </div>`;

    // Poll for activation
    if (traktPollTimer) clearInterval(traktPollTimer);
    const interval = (data.interval || 5) * 1000;
    const deviceCode = data.device_code;
    traktPollTimer = setInterval(async () => {
        const poll = await api('POST', '/trakt/activate', { device_code: deviceCode });
        if (poll.success && poll.data && poll.data.connected) {
            clearInterval(traktPollTimer); traktPollTimer = null;
            toast('Trakt.tv connected!');
            loadProfileTrakt();
        }
    }, interval);
}

function cancelTraktAuth() {
    if (traktPollTimer) { clearInterval(traktPollTimer); traktPollTimer = null; }
    loadProfileTrakt();
}

async function disconnectTrakt() {
    if (!confirm('Disconnect Trakt.tv? Scrobbling will stop.')) return;
    const res = await api('DELETE', '/trakt/disconnect');
    if (res.success) { toast('Trakt.tv disconnected'); loadProfileTrakt(); }
    else toast(res.error || 'Failed to disconnect', 'error');
}

// ──── B3: Last.fm Connection ────
async function loadProfileLastfm() {
    const area = document.getElementById('profLastfmArea');
    if (!area) return;
    try {
        const res = await api('GET', '/lastfm/status');
        const connected = res.success && res.data && res.data.connected;
        if (connected) {
            area.innerHTML = `
                <div style="display:flex;align-items:center;gap:12px;">
                    <strong style="color:#e5e5e5;">Last.fm</strong>
                    <span class="tag tag-green">Connected</span>
                    <span style="color:#8a9bae;font-size:0.82rem;">${res.data.username || ''}</span>
                    <button class="btn-danger btn-small" style="margin-left:auto;" onclick="disconnectLastfm()">Disconnect</button>
                </div>`;
        } else {
            area.innerHTML = `
                <div style="display:flex;align-items:center;gap:12px;margin-bottom:10px;">
                    <strong style="color:#e5e5e5;">Last.fm</strong>
                    <span class="tag tag-cyan" style="opacity:0.5;">Not Connected</span>
                </div>
                <p style="color:#5a6a7f;font-size:0.78rem;margin-bottom:10px;">Scrobble your music listens to Last.fm</p>
                <div style="display:flex;gap:10px;flex-wrap:wrap;">
                    <input type="text" id="lastfmUser" placeholder="Last.fm username" style="flex:1;min-width:120px;">
                    <input type="password" id="lastfmPass" placeholder="Last.fm password" style="flex:1;min-width:120px;">
                    <button class="btn-primary btn-small" onclick="connectLastfm()">Connect</button>
                </div>`;
        }
    } catch {
        area.innerHTML = '<p style="color:#5a6a7f;">Could not load Last.fm status</p>';
    }
}

async function connectLastfm() {
    const username = document.getElementById('lastfmUser')?.value;
    const password = document.getElementById('lastfmPass')?.value;
    if (!username || !password) { toast('Username and password required', 'error'); return; }
    const res = await api('POST', '/lastfm/connect', { username, password });
    if (res.success) { toast('Last.fm connected!'); loadProfileLastfm(); }
    else toast(res.error || 'Connection failed', 'error');
}

async function disconnectLastfm() {
    if (!confirm('Disconnect Last.fm? Scrobbling will stop.')) return;
    const res = await api('DELETE', '/lastfm/disconnect');
    if (res.success) { toast('Last.fm disconnected'); loadProfileLastfm(); }
    else toast(res.error || 'Failed to disconnect', 'error');
}

// ──── B4: Home Layout Customization ────
const HOME_SECTIONS = [
    { key: 'continue_watching', label: 'Continue Watching' },
    { key: 'recommendations', label: 'Recommended For You' },
    { key: 'because_you_watched', label: 'Because You Watched' },
    { key: 'on_deck', label: 'On Deck' },
    { key: 'trending', label: 'Trending' },
    { key: 'watchlist', label: 'Watchlist' },
    { key: 'favorites', label: 'Favorites' },
    { key: 'recently_added', label: 'Recently Added' },
];

async function loadProfileHomeLayout() {
    const area = document.getElementById('profHomeLayout');
    if (!area) return;
    try {
        const res = await api('GET', '/settings/home-layout');
        const layout = res.success && res.data ? res.data : {};
        const order = layout.order || HOME_SECTIONS.map(s => s.key);
        const hidden = layout.hidden || [];

        // Merge: show sections in saved order, then any new ones
        const ordered = [];
        order.forEach(key => {
            const sec = HOME_SECTIONS.find(s => s.key === key);
            if (sec) ordered.push(sec);
        });
        HOME_SECTIONS.forEach(sec => {
            if (!ordered.find(s => s.key === sec.key)) ordered.push(sec);
        });

        area.innerHTML = `
            <div class="home-layout-list" id="homeLayoutList">
                ${ordered.map((sec, idx) => {
                    const isHidden = hidden.includes(sec.key);
                    return `<div class="home-layout-item" data-key="${sec.key}">
                        <span class="home-layout-handle" title="Drag to reorder">&#9776;</span>
                        <span class="home-layout-label">${sec.label}</span>
                        <label class="toggle-label" style="margin:0;margin-left:auto;">
                            <span class="toggle-switch"><input type="checkbox" class="hl-toggle" data-key="${sec.key}" ${!isHidden?'checked':''}><span class="toggle-slider"></span></span>
                        </label>
                        <button class="btn-secondary btn-small" onclick="moveHomeSection('${sec.key}',-1)" ${idx===0?'disabled':''}>&uarr;</button>
                        <button class="btn-secondary btn-small" onclick="moveHomeSection('${sec.key}',1)" ${idx===ordered.length-1?'disabled':''}>&darr;</button>
                    </div>`;
                }).join('')}
            </div>
            <button class="btn-primary" onclick="saveHomeLayout()" style="margin-top:12px;">Save Layout</button>`;
    } catch {
        area.innerHTML = '<p style="color:#5a6a7f;">Could not load home layout</p>';
    }
}

function moveHomeSection(key, direction) {
    const list = document.getElementById('homeLayoutList');
    const items = [...list.children];
    const idx = items.findIndex(el => el.dataset.key === key);
    if (idx < 0) return;
    const newIdx = idx + direction;
    if (newIdx < 0 || newIdx >= items.length) return;
    if (direction < 0) list.insertBefore(items[idx], items[newIdx]);
    else list.insertBefore(items[newIdx], items[idx]);
    // Update button states
    const updated = [...list.children];
    updated.forEach((el, i) => {
        const btns = el.querySelectorAll('button');
        if (btns[0]) btns[0].disabled = (i === 0);
        if (btns[1]) btns[1].disabled = (i === updated.length - 1);
    });
}

async function saveHomeLayout() {
    const list = document.getElementById('homeLayoutList');
    const items = [...list.children];
    const order = items.map(el => el.dataset.key);
    const hidden = [];
    items.forEach(el => {
        const cb = el.querySelector('.hl-toggle');
        if (cb && !cb.checked) hidden.push(el.dataset.key);
    });
    const d = await api('PUT', '/settings/home-layout', { order, hidden });
    if (d.success) toast('Home layout saved!'); else toast(d.error || 'Failed to save', 'error');
}

// ──── B5: Default Streaming Quality ────
async function loadProfilePlaybackPrefs() {
    const area = document.getElementById('profPlaybackPrefs');
    if (!area) return;
    try {
        const res = await api('GET', '/settings/playback');
        const p = res.success ? (res.data || {}) : {};
        area.innerHTML = `
            <div class="form-group"><label>Default Quality</label>
                <select id="profDefQuality">
                    <option value="auto" ${(!p.preferred_quality||p.preferred_quality==='auto')?'selected':''}>Auto (Recommended)</option>
                    <option value="360p" ${p.preferred_quality==='360p'?'selected':''}>360p</option>
                    <option value="480p" ${p.preferred_quality==='480p'?'selected':''}>480p</option>
                    <option value="720p" ${p.preferred_quality==='720p'?'selected':''}>720p</option>
                    <option value="1080p" ${p.preferred_quality==='1080p'?'selected':''}>1080p</option>
                    <option value="4K" ${p.preferred_quality==='4K'?'selected':''}>4K</option>
                </select>
            </div>
            <div class="form-group"><label class="toggle-label" style="margin-bottom:0;display:flex;align-items:center;gap:10px;">
                <span class="toggle-switch"><input type="checkbox" id="profAutoPlay" ${p.auto_play_next?'checked':''}><span class="toggle-slider"></span></span>
                Auto-play next episode
            </label></div>
            <button class="btn-primary" onclick="saveProfilePlayback()">Save Playback Preferences</button>`;
    } catch {
        area.innerHTML = '<p style="color:#5a6a7f;">Could not load playback preferences</p>';
    }
}

async function saveProfilePlayback() {
    const d = await api('PUT', '/settings/playback', {
        preferred_quality: document.getElementById('profDefQuality').value,
        auto_play_next: document.getElementById('profAutoPlay').checked
    });
    if (d.success) toast('Playback preferences saved!'); else toast(d.error || 'Failed to save', 'error');
}

// ──── Country-aware Content Rating System ────

const COUNTRY_RATINGS = {
    US: [
        {value:'G', label:'G — General Audiences'},
        {value:'PG', label:'PG — Parental Guidance'},
        {value:'PG-13', label:'PG-13 — Parents Strongly Cautioned'},
        {value:'R', label:'R — Restricted'},
        {value:'NC-17', label:'NC-17 — Adults Only'}
    ],
    GB: [
        {value:'U', label:'U — Universal'},
        {value:'PG', label:'PG — Parental Guidance'},
        {value:'12A', label:'12A — 12 Accompanied'},
        {value:'15', label:'15 — Fifteen'},
        {value:'18', label:'18 — Adults Only'}
    ],
    CA: [
        {value:'G', label:'G — General'},
        {value:'PG', label:'PG — Parental Guidance'},
        {value:'14A', label:'14A — 14 Accompaniment'},
        {value:'18A', label:'18A — 18 Accompaniment'},
        {value:'R', label:'R — Restricted'}
    ],
    AU: [
        {value:'G', label:'G — General'},
        {value:'PG', label:'PG — Parental Guidance'},
        {value:'M', label:'M — Mature'},
        {value:'MA15+', label:'MA15+ — Mature Accompanied'},
        {value:'R18+', label:'R18+ — Restricted'}
    ],
    DE: [
        {value:'FSK 0', label:'FSK 0 — No restriction'},
        {value:'FSK 6', label:'FSK 6 — Ages 6+'},
        {value:'FSK 12', label:'FSK 12 — Ages 12+'},
        {value:'FSK 16', label:'FSK 16 — Ages 16+'},
        {value:'FSK 18', label:'FSK 18 — Adults Only'}
    ],
    FR: [
        {value:'U', label:'U — Tous publics'},
        {value:'-12', label:'-12 — Restricted 12'},
        {value:'-16', label:'-16 — Restricted 16'},
        {value:'-18', label:'-18 — Restricted 18'}
    ],
    JP: [
        {value:'G', label:'G — General'},
        {value:'PG12', label:'PG12 — Parental Guidance 12'},
        {value:'R15+', label:'R15+ — Ages 15+'},
        {value:'R18+', label:'R18+ — Adults Only'}
    ]
};

function updateRatingOptions() {
    const country = document.getElementById('profRatingCountry')?.value || 'US';
    const sel = document.getElementById('profMaxRating');
    if (!sel) return;
    const current = sel.value;
    const ratings = COUNTRY_RATINGS[country] || COUNTRY_RATINGS.US;
    sel.innerHTML = '<option value="">No Restriction</option>';
    ratings.forEach(r => {
        const opt = document.createElement('option');
        opt.value = r.value;
        opt.textContent = r.label;
        if (r.value === current) opt.selected = true;
        sel.appendChild(opt);
    });
}

function updateFormRatingOptions() {
    const country = document.getElementById('mpFormRatingCountry')?.value || 'US';
    const sel = document.getElementById('mpFormRating');
    if (!sel) return;
    const current = sel.value;
    const ratings = COUNTRY_RATINGS[country] || COUNTRY_RATINGS.US;
    sel.innerHTML = '<option value="">Unrestricted</option>';
    ratings.forEach(r => {
        const opt = document.createElement('option');
        opt.value = r.value;
        opt.textContent = r.label;
        if (r.value === current) opt.selected = true;
        sel.appendChild(opt);
    });
}

// ──── Phase 7: Engagement Functions ────

