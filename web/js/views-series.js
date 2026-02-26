// Movie series views
async function showLibrarySeries() {
    const libId = _gridState.libraryId;
    if (!libId) return;

    const wrapper = document.getElementById('mediaGridWrapper');
    const collArea = document.getElementById('collectionsArea');
    const serArea = document.getElementById('seriesArea');
    if (wrapper) wrapper.style.display = 'none';
    if (collArea) collArea.style.display = 'none';
    if (serArea) serArea.style.display = 'block';

    const gridBtn = document.getElementById('ftGridBtn');
    const collBtn = document.getElementById('ftCollBtn');
    const serBtn = document.getElementById('ftSeriesBtn');
    if (gridBtn) gridBtn.classList.remove('active');
    if (collBtn) collBtn.classList.remove('active');
    if (serBtn) serBtn.classList.add('active');

    serArea.innerHTML = '<div class="spinner"></div> Loading series...';
    const data = await api('GET', '/series?library_id=' + libId);
    const seriesList = (data.success && data.data) ? data.data : [];

    if (seriesList.length === 0) {
        serArea.innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#127910;</div><div class="empty-state-title">No series</div><p>Add movies to a series from the edit modal</p></div>';
        return;
    }

    serArea.innerHTML = '<div class="series-grid">' + seriesList.map(s => {
        const poster = s.poster_path
            ? `<img src="${posterSrc(s.poster_path, s.updated_at)}" alt="">`
            : '&#127910;';
        return `<div class="series-card" onclick="loadSeriesDetail('${s.id}')">
            <div class="sc-poster">${poster}</div>
            <div class="sc-info">
                <div class="sc-name">${s.name}</div>
                <div class="sc-meta">${s.item_count || 0} movie${(s.item_count||0) !== 1 ? 's' : ''}</div>
            </div>
        </div>`;
    }).join('') + '</div>';
}

async function loadSeriesDetail(seriesId) {
    _currentNav = { view: '__series', extra: seriesId };
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner"></div> Loading...';
    const data = await api('GET', '/series/' + seriesId);
    if (!data.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Series not found</div></div>'; return; }
    const s = data.data;
    const richItems = s.rich_items || [];
    const count = richItems.length || (s.items || []).length;

    // Build media card grid from rich items (edition-grouped, full badges)
    let gridHTML = '';
    if (richItems.length > 0) {
        gridHTML = '<div class="media-grid">' + richItems.map(item => {
            const card = renderMediaCard(item);
            // Inject remove-from-series overlay button into the poster area
            const siId = item.series_item_id || '';
            const removeBtn = `<button class="smi-remove-overlay" onclick="event.stopPropagation();removeFromSeries('${s.id}','${siId}')" title="Remove from series">&#10005;</button>`;
            return card.replace('<div class="play-overlay">', removeBtn + '<div class="play-overlay">');
        }).join('') + '</div>';
    } else {
        gridHTML = '<div class="empty-state"><p>No movies in this series yet</p></div>';
    }

    mc.innerHTML = `
        <div class="series-detail-header">
            <h2>${s.name}</h2>
            <span class="tag tag-cyan">${count} movie${count !== 1 ? 's' : ''}</span>
            <div class="series-detail-actions">
                <button class="btn-danger" onclick="deleteSeries('${s.id}')">&#128465; Delete Series</button>
            </div>
        </div>
        ${gridHTML}
        <button class="btn-secondary" style="margin-top:20px;" onclick="navigate('library','${s.library_id}')">&#8592; Back to Library</button>`;
}

async function deleteSeries(seriesId) {
    if (!confirm('Delete this series? Movies will not be deleted.')) return;
    const d = await api('DELETE', '/series/' + seriesId);
    if (d.success) {
        toast('Series deleted');
        history.back();
    } else {
        toast(d.error || 'Failed to delete', 'error');
    }
}

async function removeFromSeries(seriesId, itemId) {
    if (!confirm('Remove this movie from the series?')) return;
    const d = await api('DELETE', '/series/' + seriesId + '/items/' + itemId);
    if (d.success) {
        toast('Removed from series');
        loadSeriesDetail(seriesId);
    } else {
        toast(d.error || 'Failed to remove', 'error');
    }
}

// ── Add to Series Modal ──
function openSeriesModal() {
    const mediaId = document.getElementById('editMediaId').value;
    document.getElementById('seriesMediaId').value = mediaId;
    document.getElementById('seriesNameInput').value = '';
    document.getElementById('seriesSelectedId').value = '';
    document.getElementById('seriesOrderInput').value = '1';

    // Load series list for this library
    const libId = _gridState.libraryId;
    if (libId) {
        api('GET', '/series?library_id=' + libId).then(data => {
            _seriesLibraryList = (data.success && data.data) ? data.data : [];
            renderSeriesDropdown(_seriesLibraryList);
        });
    }

    document.getElementById('seriesModalOverlay').classList.add('active');
}

function closeSeriesModal() {
    document.getElementById('seriesModalOverlay').classList.remove('active');
    document.getElementById('seriesDropdown').style.display = 'none';
}

function renderSeriesDropdown(list) {
    const dd = document.getElementById('seriesDropdown');
    if (list.length === 0) {
        dd.innerHTML = '<div style="padding:10px 14px;color:#5a6a7f;font-size:0.82rem;">No existing series — type a name to create one</div>';
    } else {
        dd.innerHTML = list.map(s =>
            `<div style="padding:10px 14px;color:#b8c5d6;font-size:0.85rem;cursor:pointer;transition:background 0.15s;" onmouseenter="this.style.background='rgba(0,217,255,0.1)'" onmouseleave="this.style.background='none'" onclick="selectSeriesOption('${s.id}','${s.name.replace(/'/g,"\\'")}')">${s.name} <span style="color:#5a6a7f;font-size:0.75rem;">(${s.item_count||0} movies)</span></div>`
        ).join('');
    }
}

function showSeriesDropdown() {
    const dd = document.getElementById('seriesDropdown');
    renderSeriesDropdown(_seriesLibraryList);
    dd.style.display = 'block';
}

function filterSeriesDropdown(query) {
    const dd = document.getElementById('seriesDropdown');
    document.getElementById('seriesSelectedId').value = '';
    if (!query.trim()) {
        renderSeriesDropdown(_seriesLibraryList);
        dd.style.display = 'block';
        return;
    }
    const filtered = _seriesLibraryList.filter(s => s.name.toLowerCase().includes(query.toLowerCase()));
    if (filtered.length === 0) {
        dd.innerHTML = '<div style="padding:10px 14px;color:#00D9FF;font-size:0.82rem;">&#10010; Create "' + query + '"</div>';
    } else {
        renderSeriesDropdown(filtered);
    }
    dd.style.display = 'block';
}

function selectSeriesOption(id, name) {
    document.getElementById('seriesNameInput').value = name;
    document.getElementById('seriesSelectedId').value = id;
    document.getElementById('seriesDropdown').style.display = 'none';
}

async function saveToSeries() {
    const mediaId = document.getElementById('seriesMediaId').value;
    const name = document.getElementById('seriesNameInput').value.trim();
    let seriesId = document.getElementById('seriesSelectedId').value;
    const sortOrder = parseInt(document.getElementById('seriesOrderInput').value) || 1;

    if (!name) { toast('Series name is required', 'error'); return; }

    // If no existing series selected, create a new one
    if (!seriesId) {
        const libId = _gridState.libraryId;
        if (!libId) { toast('Library not found', 'error'); return; }
        const createRes = await api('POST', '/series', { library_id: libId, name: name });
        if (!createRes.success) { toast(createRes.error || 'Failed to create series', 'error'); return; }
        seriesId = createRes.data.id;
    }

    // Add the movie to the series
    const addRes = await api('POST', '/series/' + seriesId + '/items', {
        media_item_id: mediaId,
        sort_order: sortOrder
    });
    if (addRes.success) {
        toast('Added to series "' + name + '"');
        closeSeriesModal();
        // Refresh edit modal series info
        loadEditSeriesInfo(mediaId);
    } else {
        toast(addRes.error || 'Failed to add to series', 'error');
    }
}

async function loadEditSeriesInfo(mediaId) {
    const seriesGroup = document.getElementById('editSeriesGroup');
    const seriesInfo = document.getElementById('editSeriesInfo');
    const data = await api('GET', '/media/' + mediaId + '/series');
    if (data.success && data.data && data.data.in_series) {
        const s = data.data;
        seriesInfo.innerHTML = `<div class="series-current-info"><span class="sci-name">&#127910; ${s.series.name} — #${s.sort_order}</span><button class="sci-remove" onclick="removeMediaFromSeries('${s.series.id}','${s.item_id}','${mediaId}')">Remove</button></div>`;
    } else {
        seriesInfo.innerHTML = '';
    }
}

async function removeMediaFromSeries(seriesId, itemId, mediaId) {
    if (!confirm('Remove this movie from the series?')) return;
    const d = await api('DELETE', '/series/' + seriesId + '/items/' + itemId);
    if (d.success) {
        toast('Removed from series');
        loadEditSeriesInfo(mediaId);
    } else {
        toast(d.error || 'Failed to remove', 'error');
    }
}

// Close series dropdown on click outside
document.addEventListener('click', function(e) {
    const dd = document.getElementById('seriesDropdown');
    const inp = document.getElementById('seriesNameInput');
    if (dd && inp && !dd.contains(e.target) && e.target !== inp) {
        dd.style.display = 'none';
    }
});

// Close series modal on overlay click
document.getElementById('seriesModalOverlay').addEventListener('click', function(e) {
    if (e.target === this) closeSeriesModal();
});

// ──── Edit Profile ────
