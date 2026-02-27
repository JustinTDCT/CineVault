// Performers, tags, studios, duplicates management
async function loadPerformersView() {
    const mc=document.getElementById('mainContent'); const isAdmin=currentUser&&currentUser.role==='admin';
    mc.innerHTML=`<div class="section-header"><h2 class="section-title">Performers</h2>${isAdmin?'<button class="btn-primary" onclick="showCreatePerformer()">+ Add Performer</button>':''}</div><div class="person-grid" id="performerGrid"><div class="spinner"></div></div>`;
    const data=await api('GET','/performers');
    const grid=document.getElementById('performerGrid');
    if(data.success&&data.data&&data.data.length>0){
        grid.innerHTML=data.data.map(p=>`<div class="person-card" onclick="loadPerformerDetail('${p.id}')"><div class="person-avatar">${p.photo_path?'<img src="'+p.photo_path+'">':'&#128100;'}</div><div class="person-name">${p.name}</div><div class="person-role">${p.performer_type} \u00b7 ${p.media_count||0} media</div></div>`).join('');
    } else grid.innerHTML='<div class="empty-state" style="grid-column:1/-1;"><div class="empty-state-icon">&#128100;</div><div class="empty-state-title">No performers</div><p>Add performers to link them to your media</p></div>';
}

async function loadPerformerDetail(id) {
    const mc=document.getElementById('mainContent');
    mc.innerHTML='<div class="spinner"></div> Loading...';
    const data=await api('GET','/performers/'+id);
    if(!data.success){mc.innerHTML='<div class="empty-state"><div class="empty-state-title">Performer not found</div></div>';return;}
    const p=data.data.performer; const media=data.data.media||[];
    const isAdmin = currentUser && currentUser.role === 'admin';
    let metaRows = '';
    if (p.birth_date) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Born</td><td>${new Date(p.birth_date).toLocaleDateString()}</td></tr>`;
    if (p.birth_place) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Birthplace</td><td>${p.birth_place}</td></tr>`;
    if (p.death_date) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Died</td><td>${new Date(p.death_date).toLocaleDateString()}</td></tr>`;
    if (p.aliases) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Aliases</td><td>${p.aliases}</td></tr>`;
    if (p.nationality) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Nationality</td><td>${p.nationality}</td></tr>`;
    if (p.height) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Height</td><td>${p.height}</td></tr>`;
    if (p.tmdb_person_id) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">TMDB</td><td><a href="https://www.themoviedb.org/person/${p.tmdb_person_id}" target="_blank" style="color:#00D9FF;">#${p.tmdb_person_id}</a></td></tr>`;
    if (p.imdb_person_id) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">IMDB</td><td><a href="https://www.imdb.com/name/${p.imdb_person_id}/" target="_blank" style="color:#00D9FF;">${p.imdb_person_id}</a></td></tr>`;
    const extMeta = metaRows ? `<table style="width:100%;font-size:0.85rem;margin-top:12px;">${metaRows}</table>` : '';
    mc.innerHTML=`<div class="detail-hero"><div class="detail-poster">${p.photo_path?'<img src="'+p.photo_path+'">':'&#128100;'}</div><div class="detail-info"><h1>${p.name}</h1><div class="meta-row">${p.performer_type}${p.birth_date?' \u00b7 Born: '+new Date(p.birth_date).toLocaleDateString():''}</div>${p.bio?'<p class="description">'+p.bio+'</p>':''}<span class="tag tag-cyan">${p.media_count||0} media items</span>${extMeta}${isAdmin ? '<div style="margin-top:12px;display:flex;gap:8px;"><button class="btn-secondary btn-small" onclick="showEditPerformer(\''+id+'\')">&#9998; Edit</button><button class="btn-danger btn-small" onclick="deletePerformer(\''+id+'\')">Delete</button></div>' : ''}</div></div>
    ${media.length>0?'<h3 style="color:#00D9FF;margin-bottom:16px;">Linked Media</h3><div class="media-grid">'+media.map(renderMediaCard).join('')+'</div>':''}
    <button class="btn-secondary" onclick="loadPerformersView()">&#8592; Back</button>`;
}

function showCreatePerformer(){const mc=document.getElementById('mainContent');mc.innerHTML=`<div class="section-header"><h2 class="section-title">Add Performer</h2></div><div style="max-width:500px;"><div class="form-group"><label>Name</label><input type="text" id="perfName"></div><div class="form-group"><label>Type</label><select id="perfType"><option value="actor">Actor</option><option value="director">Director</option><option value="producer">Producer</option><option value="musician">Musician</option><option value="narrator">Narrator</option><option value="adult_performer">Adult Performer</option><option value="other">Other</option></select></div><div class="form-group"><label>Bio</label><textarea id="perfBio" rows="3"></textarea></div><button class="btn-primary" onclick="createPerformer()">Create</button><button class="btn-secondary" style="margin-left:12px;" onclick="loadPerformersView()">Cancel</button></div>`;}
async function createPerformer(){const n=document.getElementById('perfName').value,t=document.getElementById('perfType').value,b=document.getElementById('perfBio').value||null;if(!n){toast('Name required','error');return;}const d=await api('POST','/performers',{name:n,performer_type:t,bio:b});if(d.success){toast('Created!');loadPerformersView();}else toast(d.error,'error');}

async function deletePerformer(id) {
    if (!confirm('Delete this performer?')) return;
    const d = await api('DELETE', '/performers/' + id);
    if (d.success) { toast('Performer deleted'); loadPerformersView(); }
    else toast(d.error, 'error');
}

async function showEditPerformer(id) {
    const res = await api('GET', '/performers/' + id);
    if (!res.success) { toast('Failed to load performer', 'error'); return; }
    const p = res.data.performer;
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Edit Performer</h2></div>
        <div style="max-width:560px;">
            <div class="form-group"><label>Name</label><input type="text" id="epName" value="${p.name || ''}"></div>
            <div class="form-group"><label>Type</label><select id="epType"><option value="actor" ${p.performer_type==='actor'?'selected':''}>Actor</option><option value="director" ${p.performer_type==='director'?'selected':''}>Director</option><option value="producer" ${p.performer_type==='producer'?'selected':''}>Producer</option><option value="musician" ${p.performer_type==='musician'?'selected':''}>Musician</option><option value="narrator" ${p.performer_type==='narrator'?'selected':''}>Narrator</option><option value="adult_performer" ${p.performer_type==='adult_performer'?'selected':''}>Adult Performer</option><option value="other" ${p.performer_type==='other'?'selected':''}>Other</option></select></div>
            <div class="form-group"><label>Bio</label><textarea id="epBio" rows="3">${p.bio || ''}</textarea></div>
            <div class="form-group"><label>Photo URL</label><input type="text" id="epPhoto" value="${p.photo_path || ''}" placeholder="URL to photo"></div>
            <div class="edit-field-row">
                <div class="form-group"><label>Birth Date</label><input type="date" id="epBirthDate" value="${p.birth_date ? p.birth_date.substring(0,10) : ''}"></div>
                <div class="form-group"><label>Birth Place</label><input type="text" id="epBirthPlace" value="${p.birth_place || ''}"></div>
            </div>
            <div class="edit-field-row">
                <div class="form-group"><label>Nationality</label><input type="text" id="epNationality" value="${p.nationality || ''}"></div>
                <div class="form-group"><label>Aliases</label><input type="text" id="epAliases" value="${p.aliases || ''}" placeholder="Comma-separated"></div>
            </div>
            <button class="btn-primary" onclick="savePerformerEdit('${id}')">Save Changes</button>
            <button class="btn-secondary" style="margin-left:12px;" onclick="loadPerformerDetail('${id}')">Cancel</button>
        </div>`;
}

async function savePerformerEdit(id) {
    const name = document.getElementById('epName').value.trim();
    if (!name) { toast('Name required', 'error'); return; }
    const d = await api('PUT', '/performers/' + id, {
        name,
        performer_type: document.getElementById('epType').value,
        bio: document.getElementById('epBio').value.trim() || null,
        photo_path: document.getElementById('epPhoto').value.trim() || null,
        birth_date: document.getElementById('epBirthDate').value || null,
        birth_place: document.getElementById('epBirthPlace').value.trim() || null,
        nationality: document.getElementById('epNationality').value.trim() || null,
        aliases: document.getElementById('epAliases').value.trim() || null
    });
    if (d.success) { toast('Performer updated'); loadPerformerDetail(id); }
    else toast(d.error, 'error');
}

// ──── Tags ────
async function loadTagsView() {
    const mc=document.getElementById('mainContent'); const isAdmin=currentUser&&currentUser.role==='admin';
    mc.innerHTML=`<div class="section-header"><h2 class="section-title">Tags & Genres</h2>${isAdmin?'<button class="btn-primary" onclick="showCreateTag()">+ Add Tag</button>':''}</div><div id="tagsList"><div class="spinner"></div></div>`;
    const data=await api('GET','/tags?tree=true');
    const div=document.getElementById('tagsList');
    if(data.success&&data.data&&data.data.length>0){
        div.innerHTML=data.data.map(t=>renderTag(t,0)).join('');
    } else div.innerHTML='<div class="empty-state"><div class="empty-state-icon">&#127991;</div><div class="empty-state-title">No tags</div></div>';
}

function renderTag(tag, depth) {
    const indent = depth * 24;
    const isAdmin = currentUser && currentUser.role === 'admin';
    let html = `<div class="group-card" style="margin-left:${indent}px;display:flex;justify-content:space-between;align-items:center;"><div><h4>${tag.name}</h4>${tag.description ? '<p style="color:#5a6a7f;font-size:0.78rem;margin:2px 0;">'+tag.description+'</p>' : ''}<span class="tag tag-${tag.category==='genre'?'purple':tag.category==='custom'?'orange':'cyan'}">${tag.category}</span><span class="tag tag-green">${tag.media_count||0} media</span></div><div style="display:flex;gap:6px;">${isAdmin ? '<button class="btn-secondary btn-small" onclick="showEditTag(\''+tag.id+'\',\''+tag.name.replace(/'/g,"\\'")+'\',\''+tag.category+'\',\''+(tag.description||'').replace(/'/g,"\\'")+'\')">&#9998;</button><button class="btn-danger btn-small" onclick="deleteTag(\''+tag.id+'\')">Delete</button>' : ''}</div></div>`;
    if (tag.children) tag.children.forEach(c => html += renderTag(c, depth+1));
    return html;
}

function showCreateTag(){const mc=document.getElementById('mainContent');mc.innerHTML=`<div class="section-header"><h2 class="section-title">Add Tag</h2></div><div style="max-width:500px;"><div class="form-group"><label>Name</label><input type="text" id="tagName"></div><div class="form-group"><label>Category</label><select id="tagCat"><option value="genre">Genre</option><option value="tag">Tag</option><option value="custom">Custom</option></select></div><div class="form-group"><label>Description</label><input type="text" id="tagDesc"></div><button class="btn-primary" onclick="createTag()">Create</button><button class="btn-secondary" style="margin-left:12px;" onclick="loadTagsView()">Cancel</button></div>`;}
async function createTag(){const n=document.getElementById('tagName').value,c=document.getElementById('tagCat').value,d=document.getElementById('tagDesc').value||null;if(!n){toast('Name required','error');return;}const r=await api('POST','/tags',{name:n,category:c,description:d});if(r.success){toast('Created!');loadTagsView();}else toast(r.error,'error');}
async function deleteTag(id){if(!confirm('Delete?'))return;const d=await api('DELETE','/tags/'+id);if(d.success){toast('Deleted');loadTagsView();}else toast(d.error,'error');}

function showEditTag(id, name, category, description) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Edit Tag</h2></div>
        <div style="max-width:500px;">
            <div class="form-group"><label>Name</label><input type="text" id="editTagName" value="${name}"></div>
            <div class="form-group"><label>Category</label><select id="editTagCat"><option value="genre" ${category==='genre'?'selected':''}>Genre</option><option value="tag" ${category==='tag'?'selected':''}>Tag</option><option value="custom" ${category==='custom'?'selected':''}>Custom</option></select></div>
            <div class="form-group"><label>Description</label><input type="text" id="editTagDesc" value="${description}"></div>
            <button class="btn-primary" onclick="saveTagEdit('${id}')">Save</button>
            <button class="btn-secondary" style="margin-left:12px;" onclick="loadTagsView()">Cancel</button>
        </div>`;
}

async function saveTagEdit(id) {
    const name = document.getElementById('editTagName').value.trim();
    if (!name) { toast('Name required', 'error'); return; }
    const d = await api('PUT', '/tags/' + id, {
        name,
        category: document.getElementById('editTagCat').value,
        description: document.getElementById('editTagDesc').value.trim() || null
    });
    if (d.success) { toast('Tag updated'); loadTagsView(); }
    else toast(d.error, 'error');
}

async function addTagToMedia(mediaId) {
    const sel = document.getElementById('tagAssignSelect');
    if (!sel || !sel.value) { toast('Select a tag first', 'error'); return; }
    const d = await api('POST', '/media/' + mediaId + '/tags/' + sel.value);
    if (d.success) { toast('Tag added'); showDetailTab(document.querySelector('.detail-tab.active') || document.querySelector('.detail-tab'), 'tags-tab', mediaId); }
    else toast(d.error || 'Failed to add tag', 'error');
}

async function removeTagFromMedia(mediaId, tagId) {
    const d = await api('DELETE', '/media/' + mediaId + '/tags/' + tagId);
    if (d.success) { toast('Tag removed'); showDetailTab(document.querySelector('.detail-tab.active') || document.querySelector('.detail-tab'), 'tags-tab', mediaId); }
    else toast(d.error || 'Failed to remove tag', 'error');
}

// ──── Studios ────
async function loadStudiosView() {
    const mc=document.getElementById('mainContent'); const isAdmin=currentUser&&currentUser.role==='admin';
    mc.innerHTML=`<div class="section-header"><h2 class="section-title">Studios / Labels</h2>${isAdmin?'<button class="btn-primary" onclick="showCreateStudio()">+ Add Studio</button>':''}</div><div id="studiosList"><div class="spinner"></div></div>`;
    const data=await api('GET','/studios');
    const div=document.getElementById('studiosList');
    if(data.success&&data.data&&data.data.length>0){
        div.innerHTML=data.data.map(s=>`<div class="group-card" style="cursor:pointer;" onclick="loadStudioDetail('${s.id}')"><div style="display:flex;justify-content:space-between;align-items:center;"><div><h4>${s.name}</h4>${s.website ? '<a href="'+s.website+'" target="_blank" style="color:#00D9FF;font-size:0.78rem;" onclick="event.stopPropagation();">'+s.website+'</a>' : ''}<div style="margin-top:4px;"><span class="tag tag-cyan">${s.studio_type}</span><span class="tag tag-green">${s.media_count||0} media</span></div></div><div style="display:flex;gap:6px;">${isAdmin?'<button class="btn-secondary btn-small" onclick="event.stopPropagation();showEditStudio(\''+s.id+'\',\''+s.name.replace(/'/g,"\\'")+'\',\''+s.studio_type+'\',\''+(s.website||'').replace(/'/g,"\\'")+'\')">&#9998;</button><button class="btn-danger btn-small" onclick="event.stopPropagation();deleteStudio(\''+s.id+'\')">Delete</button>':''}</div></div></div>`).join('');
    } else div.innerHTML='<div class="empty-state"><div class="empty-state-icon">&#127980;</div><div class="empty-state-title">No studios</div></div>';
}
function showCreateStudio(){const mc=document.getElementById('mainContent');mc.innerHTML=`<div class="section-header"><h2 class="section-title">Add Studio</h2></div><div style="max-width:500px;"><div class="form-group"><label>Name</label><input type="text" id="studioName"></div><div class="form-group"><label>Type</label><select id="studioType"><option value="studio">Studio</option><option value="label">Label</option><option value="publisher">Publisher</option><option value="network">Network</option><option value="distributor">Distributor</option></select></div><div class="form-group"><label>Website</label><input type="text" id="studioWeb"></div><button class="btn-primary" onclick="createStudio()">Create</button><button class="btn-secondary" style="margin-left:12px;" onclick="loadStudiosView()">Cancel</button></div>`;}
async function createStudio(){const n=document.getElementById('studioName').value,t=document.getElementById('studioType').value,w=document.getElementById('studioWeb').value||null;if(!n){toast('Name required','error');return;}const d=await api('POST','/studios',{name:n,studio_type:t,website:w});if(d.success){toast('Created!');loadStudiosView();}else toast(d.error,'error');}
async function deleteStudio(id){if(!confirm('Delete?'))return;const d=await api('DELETE','/studios/'+id);if(d.success){toast('Deleted');loadStudiosView();}else toast(d.error,'error');}

async function loadStudioDetail(id) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner"></div>';
    const data = await api('GET', '/studios/' + id);
    if (!data.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Studio not found</div></div>'; return; }
    const s = data.data.studio || data.data;
    const media = data.data.media || [];
    const isAdmin = currentUser && currentUser.role === 'admin';
    mc.innerHTML = `<div class="detail-hero"><div class="detail-poster" style="font-size:3rem;">&#127980;</div>
        <div class="detail-info"><h1>${s.name}</h1>
            <div class="meta-row">${s.studio_type}${s.website ? ' &middot; <a href="'+s.website+'" target="_blank" style="color:#00D9FF;">'+s.website+'</a>' : ''}</div>
            <span class="tag tag-cyan">${s.media_count || media.length} media items</span>
            ${isAdmin ? '<div style="margin-top:12px;display:flex;gap:8px;"><button class="btn-secondary btn-small" onclick="showEditStudio(\''+id+'\',\''+s.name.replace(/'/g,"\\'")+'\',\''+s.studio_type+'\',\''+(s.website||'').replace(/'/g,"\\'")+'\')">&#9998; Edit</button><button class="btn-danger btn-small" onclick="deleteStudio(\''+id+'\')">Delete</button></div>' : ''}
        </div></div>
    ${media.length > 0 ? '<h3 style="color:#00D9FF;margin-bottom:16px;">Media</h3><div class="media-grid">' + media.map(renderMediaCard).join('') + '</div>' : ''}
    <button class="btn-secondary" onclick="loadStudiosView()">&#8592; Back</button>`;
}

function showEditStudio(id, name, studioType, website) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Edit Studio</h2></div>
        <div style="max-width:500px;">
            <div class="form-group"><label>Name</label><input type="text" id="editStudioName" value="${name}"></div>
            <div class="form-group"><label>Type</label><select id="editStudioType"><option value="studio" ${studioType==='studio'?'selected':''}>Studio</option><option value="label" ${studioType==='label'?'selected':''}>Label</option><option value="publisher" ${studioType==='publisher'?'selected':''}>Publisher</option><option value="network" ${studioType==='network'?'selected':''}>Network</option><option value="distributor" ${studioType==='distributor'?'selected':''}>Distributor</option></select></div>
            <div class="form-group"><label>Website</label><input type="text" id="editStudioWeb" value="${website}" placeholder="https://..."></div>
            <button class="btn-primary" onclick="saveStudioEdit('${id}')">Save</button>
            <button class="btn-secondary" style="margin-left:12px;" onclick="loadStudiosView()">Cancel</button>
        </div>`;
}

async function saveStudioEdit(id) {
    const name = document.getElementById('editStudioName').value.trim();
    if (!name) { toast('Name required', 'error'); return; }
    const d = await api('PUT', '/studios/' + id, {
        name,
        studio_type: document.getElementById('editStudioType').value,
        website: document.getElementById('editStudioWeb').value.trim() || null
    });
    if (d.success) { toast('Studio updated'); loadStudiosView(); }
    else toast(d.error, 'error');
}

// ──── Duplicates ────
async function loadDuplicatesView() {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Duplicate Review</h2></div>
        <p style="color:#8a9bae;margin-bottom:20px;">Review media items flagged as exact (MD5) or potential (phash) duplicates</p>
        <div id="dupList"><div class="spinner"></div></div>`;
    const data = await api('GET', '/duplicates');
    const div = document.getElementById('dupList');
    if (!data.success) { div.innerHTML = '<p style="color:#ff6b6b;">Failed to load duplicates</p>'; return; }
    const groups = data.data && data.data.groups ? data.data.groups : [];
    if (groups.length === 0) {
        div.innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#128257;</div><div class="empty-state-title">No duplicates found</div><p>Scan libraries to detect duplicates via MD5 and perceptual hashing</p></div>';
        return;
    }
    div.innerHTML = groups.map(g => renderDuplicateGroup(g)).join('');
}

function renderDuplicateGroup(g) {
    const item = g.item;
    const typeClass = g.dup_type === 'exact' ? 'dup-type-exact' : 'dup-type-potential';
    const typeLabel = g.dup_type === 'exact' ? 'Exact Duplicate' : 'Potential Duplicate';
    const matches = g.matches || [];
    const decisions = g.decisions || [];

    let decisionsHtml = '';
    if (decisions.length > 0) {
        decisionsHtml = `<div class="dup-decisions"><strong>Prior decisions:</strong> ` +
            decisions.map(d => `${d.action} by ${d.decided_by} on ${new Date(d.decided_at).toLocaleDateString()}`
                + (d.notes ? ' - ' + d.notes : '')).join('; ') + '</div>';
    }

    let matchesHtml = '';
    if (matches.length > 0) {
        matchesHtml = `<div class="dup-compare">` + renderDupCard(item, 'Source') +
            matches.map(m => renderDupCard(m.item, m.match_type === 'md5' ?
                `<span class="dup-match-badge dup-match-md5">MD5 Match</span>` :
                `<span class="dup-match-badge dup-match-phash">${Math.round(m.similarity * 100)}% Similar</span>`
            )).join('') + `</div>`;
    }

    const bestMatch = matches.length > 0 ? matches[0] : null;
    const partnerId = bestMatch ? bestMatch.item.id : '';
    const partnerTitle = bestMatch ? bestMatch.item.title : '';

    return `<div class="dup-group">
        <div class="dup-group-header">
            <h3 style="color:#e5e5e5;">${item.title}${item.year ? ' (' + item.year + ')' : ''}</h3>
            <span class="dup-type-badge ${typeClass}">${typeLabel}</span>
        </div>
        ${matchesHtml}
        ${decisionsHtml}
        <div class="dup-actions">
            <button class="btn-edit btn-small" onclick="dupEdit('${item.id}','${partnerId}')">&#9998; Edit</button>
            <button class="btn-edition btn-small" onclick="dupMergeEdition('${item.id}','${partnerId}','${item.title.replace(/'/g,"\\'")}','${partnerTitle.replace(/'/g,"\\'")}')">&#128191; Merge as Edition</button>
            <button class="btn-danger btn-small" onclick="dupDelete('${item.id}','${partnerId}')">&#128465; Delete</button>
            <button class="btn-secondary btn-small" onclick="dupIgnore('${item.id}','${partnerId}')">&#128683; Ignore</button>
        </div>
    </div>`;
}

function renderDupCard(item, label) {
    const fileSize = item.file_size ? (item.file_size / (1024*1024*1024)).toFixed(2) + ' GB' : '';
    const res = item.resolution || '';
    const filePath = item.file_path || '';
    const shortPath = filePath.length > 60 ? '...' + filePath.slice(-57) : filePath;
    return `<div class="dup-card">
        <div class="dup-card-header">
            <div class="dup-card-poster">${item.poster_path ? '<img src="'+posterSrc(item.poster_path, item.updated_at)+'">' : '<div style="display:flex;align-items:center;justify-content:center;height:100%;font-size:2rem;color:#4a5568;">&#127910;</div>'}</div>
            <div class="dup-card-meta">
                <h4>${item.title}</h4>
                <p>${item.year || 'N/A'}${res ? ' &middot; ' + res : ''}${fileSize ? ' &middot; ' + fileSize : ''}</p>
                <p title="${filePath}" style="word-break:break-all;">${shortPath}</p>
                <div style="margin-top:6px;">${label}</div>
            </div>
        </div>
    </div>`;
}

async function dupEdit(itemId, partnerId) {
    // Open the edit modal, then mark as addressed on save
    openEditModal(itemId);
    // After save, mark both as addressed
    window._dupResolveAfterEdit = { itemId, partnerId };
}

async function dupIgnore(itemId, partnerId) {
    const d = await api('POST', '/duplicates/resolve', { media_id: itemId, partner_id: partnerId, action: 'ignored' });
    if (d.success) { toast('Marked as ignored'); loadDuplicatesView(); loadSidebarCounts(); } else toast(d.error, 'error');
}

async function dupDelete(itemId, partnerId) {
    if (!confirm('Delete this item from the database?')) return;
    const deleteFile = confirm('Also delete the file from disk? (Cannot be undone)');
    const d = await api('POST', '/duplicates/resolve', { media_id: itemId, partner_id: partnerId, action: 'deleted', delete_file: deleteFile });
    if (d.success) { toast('Item deleted'); loadDuplicatesView(); loadSidebarCounts(); } else toast(d.error, 'error');
}

function dupMergeEdition(itemA, itemB, titleA, titleB) {
    document.getElementById('mergeItemA').value = itemA;
    document.getElementById('mergeItemB').value = itemB;
    const opts = document.getElementById('mergePrimaryOptions');
    opts.innerHTML = `
        <div class="merge-option selected" onclick="selectMergePrimary(this,'${itemA}')">
            <label><input type="radio" name="mergePrimary" value="${itemA}" checked> ${titleA} (Source)</label>
        </div>
        <div class="merge-option" onclick="selectMergePrimary(this,'${itemB}')">
            <label><input type="radio" name="mergePrimary" value="${itemB}"> ${titleB} (Match)</label>
        </div>`;
    document.getElementById('mergeEditionOverlay').classList.add('active');
}

function selectMergePrimary(el, id) {
    document.querySelectorAll('.merge-option').forEach(o => o.classList.remove('selected'));
    el.classList.add('selected');
    el.querySelector('input[type="radio"]').checked = true;
}

function closeMergeModal() {
    document.getElementById('mergeEditionOverlay').classList.remove('active');
}

async function submitMergeEdition() {
    const itemA = document.getElementById('mergeItemA').value;
    const itemB = document.getElementById('mergeItemB').value;
    const label = document.getElementById('mergeEditionLabel').value;
    const primaryId = document.querySelector('input[name="mergePrimary"]:checked').value;
    const d = await api('POST', '/duplicates/resolve', {
        media_id: itemA, partner_id: itemB, action: 'edition',
        edition_label: label, primary_id: primaryId
    });
    if (d.success) { toast('Merged as edition!'); closeMergeModal(); loadDuplicatesView(); loadSidebarCounts(); }
    else toast(d.error, 'error');
}

// Close merge modal on overlay click
document.getElementById('mergeEditionOverlay').addEventListener('click', function(e) {
    if (e.target === this) closeMergeModal();
});

// ──── Metadata Identify ────
let _identifyMatches = [];
let _identifyMediaId = '';

async function identifyMedia(id) {
    const mc=document.getElementById('mainContent');
    mc.innerHTML='<div class="section-header"><h2 class="section-title">Identify Media</h2></div><div id="matchList"><div class="spinner"></div> Searching external sources...</div>';
    const data=await api('POST','/media/'+id+'/identify');
    const div=document.getElementById('matchList');
    _identifyMediaId = id;
    _identifyMatches = (data.success && data.data) ? data.data : [];
    if(_identifyMatches.length>0){
        const esc = (s) => s ? s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;') : '';
        div.innerHTML=_identifyMatches.map((m,i)=>`<div class="group-card"><div style="display:flex;gap:16px;align-items:start;"><div style="width:80px;min-width:80px;aspect-ratio:2/3;border-radius:10px;overflow:hidden;background:rgba(0,0,0,0.3);">${m.poster_url?'<img src="'+m.poster_url+'" style="width:100%;height:100%;object-fit:cover;">':'&#128247;'}</div><div><h4>${esc(m.title)}${m.year?' ('+m.year+')':''}</h4><p>${m.description?esc(m.description.substring(0,200))+'...':''}</p><span class="tag tag-cyan">${m.source}</span><span class="tag tag-green">${Math.round(m.confidence*100)}% match</span><button class="btn-primary btn-small" style="margin-top:8px;" onclick="applyMatchByIndex(${i})">Apply</button></div></div></div>`).join('');
    } else div.innerHTML='<div class="empty-state"><div class="empty-state-title">No matches found</div></div>';
    div.innerHTML+='<button class="btn-secondary" style="margin-top:16px;" onclick="loadMediaDetail(\''+id+'\')">&#8592; Back</button>';
}

async function applyMatchByIndex(idx) {
    const match = _identifyMatches[idx];
    if (!match) { toast('Match not found','error'); return; }
    const d=await api('POST','/media/'+_identifyMediaId+'/apply-meta',match);
    if(d.success){toast('Metadata applied!');loadMediaDetail(_identifyMediaId);}else toast(d.error,'error');
}

// ──── Toggle Known Edition Details ────
