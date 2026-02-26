// Music views
    const wrapper = document.getElementById('mediaGridWrapper');
    if (wrapper) wrapper.style.display = 'flex';
    ['collectionsArea','seriesArea','artistsArea','albumsArea','genresArea'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.style.display = 'none';
    });
    ['ftGridBtn','ftCollBtn','ftSeriesBtn','ftArtistBtn','ftAlbumBtn','ftGenreBtn'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.toggle('active', id === 'ftGridBtn');
    });
}

// ──── Music: View by Artist / Album ────
function renderArtistCard(artist) {
    const meta = [];
    if (artist.album_count) meta.push(artist.album_count + ' album' + (artist.album_count !== 1 ? 's' : ''));
    if (artist.track_count) meta.push(artist.track_count + ' track' + (artist.track_count !== 1 ? 's' : ''));
    return `<div class="media-card" tabindex="0" data-artist-id="${artist.id}" onclick="showArtistDetail('${artist.id}')">
        <div class="media-poster music-poster-round">
            ${artist.poster_path ? '<img src="'+posterSrc(artist.poster_path, artist.updated_at)+'" alt="" loading="lazy">' : '<div class="music-icon-placeholder">&#127908;</div>'}
        </div>
        <div class="media-info"><div class="media-title">${escapeHtml(artist.name)}</div><div class="media-meta">${meta.join(' \u00b7 ')}</div></div>
    </div>`;
}

function renderAlbumCard(album) {
    const meta = [];
    if (album.artist_name) meta.push(album.artist_name);
    if (album.year) meta.push(album.year);
    if (album.track_count) meta.push(album.track_count + ' track' + (album.track_count !== 1 ? 's' : ''));
    return `<div class="media-card" tabindex="0" data-album-id="${album.id}" onclick="showAlbumDetail('${album.id}')">
        <div class="media-poster">
            ${album.poster_path ? '<img src="'+posterSrc(album.poster_path, album.updated_at)+'" alt="" loading="lazy">' : '<div class="music-icon-placeholder">&#128191;</div>'}
        </div>
        <div class="media-info"><div class="media-title">${escapeHtml(album.title)}</div><div class="media-meta">${meta.join(' \u00b7 ')}</div></div>
    </div>`;
}

async function showMusicArtists() {
    const libId = _gridState.libraryId;
    if (!libId) return;

    const wrapper = document.getElementById('mediaGridWrapper');
    if (wrapper) wrapper.style.display = 'none';
    ['artistsArea','albumsArea','genresArea'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.style.display = id === 'artistsArea' ? 'block' : 'none';
    });

    ['ftGridBtn','ftArtistBtn','ftAlbumBtn','ftGenreBtn'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.toggle('active', id === 'ftArtistBtn');
    });

    artistsArea.innerHTML = '<div class="spinner"></div> Loading artists...';
    const data = await api('GET', '/libraries/' + libId + '/artists');
    const artists = (data.success && data.data) ? data.data : [];

    if (artists.length > 0) {
        artistsArea.innerHTML = `<div class="media-grid-wrapper"><div class="media-grid music-artist-grid">${artists.map(renderArtistCard).join('')}</div></div>`;
    } else {
        artistsArea.innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#127908;</div><div class="empty-state-title">No artists found</div><p>Scan the library to detect artists</p></div>';
    }
}

async function showMusicAlbums() {
    const libId = _gridState.libraryId;
    if (!libId) return;

    const wrapper = document.getElementById('mediaGridWrapper');
    if (wrapper) wrapper.style.display = 'none';
    ['artistsArea','albumsArea','genresArea'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.style.display = id === 'albumsArea' ? 'block' : 'none';
    });

    ['ftGridBtn','ftArtistBtn','ftAlbumBtn','ftGenreBtn'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.toggle('active', id === 'ftAlbumBtn');
    });

    albumsArea.innerHTML = '<div class="spinner"></div> Loading albums...';
    const data = await api('GET', '/libraries/' + libId + '/albums');
    const albums = (data.success && data.data) ? data.data : [];

    if (albums.length > 0) {
        albumsArea.innerHTML = `<div class="media-grid-wrapper"><div class="media-grid">${albums.map(renderAlbumCard).join('')}</div></div>`;
    } else {
        albumsArea.innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#128191;</div><div class="empty-state-title">No albums found</div><p>Scan the library to detect albums</p></div>';
    }
}

async function showMusicGenres() {
    const libId = _gridState.libraryId;
    if (!libId) return;

    const wrapper = document.getElementById('mediaGridWrapper');
    if (wrapper) wrapper.style.display = 'none';
    ['artistsArea','albumsArea','genresArea'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.style.display = id === 'genresArea' ? 'block' : 'none';
    });

    ['ftGridBtn','ftArtistBtn','ftAlbumBtn','ftGenreBtn'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.toggle('active', id === 'ftGenreBtn');
    });

    const genresArea = document.getElementById('genresArea');
    genresArea.innerHTML = '<div class="spinner"></div> Loading genres...';
    const data = await api('GET', '/libraries/' + libId + '/music-genres');
    const genres = (data.success && data.data) ? data.data : [];

    if (genres.length > 0) {
        let html = '<div class="media-grid-wrapper"><div class="media-grid genre-grid">';
        for (const g of genres) {
            html += `<div class="media-card genre-card" tabindex="0" onclick="filterByGenre('${g.id}','${escapeHtml(g.name)}')">
                <div class="genre-card-icon">&#127926;</div>
                <div class="media-info">
                    <div class="media-title">${escapeHtml(g.name)}</div>
                    <div class="media-meta">${g.media_count || 0} track${(g.media_count || 0) !== 1 ? 's' : ''}</div>
                </div>
            </div>`;
        }
        html += '</div></div>';
        genresArea.innerHTML = html;
    } else {
        genresArea.innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#127926;</div><div class="empty-state-title">No genres found</div><p>Scan the library to detect genres from embedded tags</p></div>';
    }
}

function filterByGenre(genreId, genreName) {
    const libId = _gridState.libraryId;
    if (!libId) return;
    const wrapper = document.getElementById('mediaGridWrapper');
    if (wrapper) wrapper.style.display = 'flex';
    ['artistsArea','albumsArea','genresArea'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.style.display = 'none';
    });
    ['ftGridBtn','ftArtistBtn','ftAlbumBtn','ftGenreBtn'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.remove('active');
    });
    const genreBtn = document.getElementById('ftGenreBtn');
    if (genreBtn) genreBtn.classList.add('active');
    _gridState.activeFilter = { genre: genreId };
    _gridState.filterLabel = 'Genre: ' + genreName;
    loadLibraryView(libId);
}

async function showArtistDetail(artistId) {
    const libId = _gridState.libraryId;
    if (!libId) return;
    _detailReturnNav = { view: 'library', extra: libId };

    const [artistData, albumsData] = await Promise.all([
        api('GET', '/artists/' + artistId),
        api('GET', '/artists/' + artistId + '/albums')
    ]);
    const artist = (artistData.success && artistData.data) ? artistData.data : null;
    const albums = (albumsData.success && albumsData.data) ? albumsData.data : [];
    const artistName = artist ? artist.name : 'Unknown Artist';

    const mc = document.getElementById('mainContent');
    const posterHtml = artist && artist.poster_path
        ? `<img src="${posterSrc(artist.poster_path, artist.updated_at)}" alt="" class="artist-detail-poster">`
        : '';
    const descHtml = artist && artist.description
        ? `<p class="artist-description">${escapeHtml(artist.description)}</p>`
        : '';
    let html = `<div class="artist-detail-header">
        <button class="btn-secondary" onclick="navigate('library','${libId}')" style="margin-right:12px;">&#8592; Back</button>
        ${posterHtml}
        <div class="artist-detail-info">
            <h2 class="section-title">${escapeHtml(artistName)}</h2>
            <span class="tag" style="margin-left:8px;">${albums.length} album${albums.length !== 1 ? 's' : ''}</span>
            <div class="artist-detail-actions">
                <button class="btn-primary" onclick="playArtist('${artistId}')">&#9654; Play All</button>
                <button class="btn-secondary" onclick="shuffleArtist('${artistId}')">&#128256; Shuffle</button>
            </div>
            ${descHtml}
        </div>
    </div>`;

    if (albums.length > 0) {
        html += `<div class="media-grid">${albums.map(renderAlbumCard).join('')}</div>`;
    } else {
        html += '<div class="empty-state"><div class="empty-state-icon">&#128191;</div><div class="empty-state-title">No albums</div></div>';
    }

    mc.innerHTML = html;
}

async function showAlbumDetail(albumId) {
    const libId = _gridState.libraryId;
    if (!libId) return;

    const [albumData, tracksData] = await Promise.all([
        api('GET', '/albums/' + albumId),
        api('GET', '/albums/' + albumId + '/tracks')
    ]);
    const album = (albumData.success && albumData.data) ? albumData.data : null;
    const tracks = (tracksData.success && tracksData.data) ? tracksData.data : [];
    const albumTitle = album ? album.title : 'Unknown Album';
    const artistName = album ? album.artist_name : '';
    const totalDuration = tracks.reduce((sum, t) => sum + (t.duration_seconds || 0), 0);

    const mc = document.getElementById('mainContent');
    const posterHtml = album && album.poster_path
        ? `<img src="${posterSrc(album.poster_path, album.updated_at)}" alt="" class="album-detail-poster">`
        : '<div class="album-detail-poster-placeholder">&#128191;</div>';

    let html = `<div class="album-detail-header">
        <button class="btn-secondary" onclick="navigate('library','${libId}')" style="margin-right:12px;">&#8592; Back</button>
        ${posterHtml}
        <div class="album-detail-info">
            <h2 class="section-title">${escapeHtml(albumTitle)}</h2>
            ${artistName ? '<span class="tag tag-cyan">'+escapeHtml(artistName)+'</span>' : ''}
            ${album && album.year ? '<span class="tag">'+album.year+'</span>' : ''}
            <span class="tag">${tracks.length} track${tracks.length !== 1 ? 's' : ''}</span>
            <span class="tag">${formatDuration(totalDuration)}</span>
            <div class="album-detail-actions">
                <button class="btn-primary" onclick="playAlbumTracks('${albumId}')">&#9654; Play All</button>
                <button class="btn-secondary" onclick="shuffleAlbumTracks('${albumId}')">&#128256; Shuffle</button>
            </div>
        </div>
    </div>`;

    if (tracks.length > 0) {
        html += renderTracklist(tracks);
    } else {
        html += '<div class="empty-state"><div class="empty-state-icon">&#127925;</div><div class="empty-state-title">No tracks</div></div>';
    }

    mc.innerHTML = html;
}

function renderTracklist(tracks) {
    let html = '<table class="tracklist-table"><thead><tr>';
    html += '<th class="tl-num">#</th><th class="tl-title">Title</th>';
    html += '<th class="tl-artist">Artist</th><th class="tl-duration">Duration</th>';
    html += '<th class="tl-play"></th></tr></thead><tbody>';
    let lastDisc = null;
    for (const t of tracks) {
        if (t.disc_number && t.disc_number !== lastDisc && tracks.some(x => x.disc_number && x.disc_number > 1)) {
            lastDisc = t.disc_number;
            html += `<tr class="tl-disc-header"><td colspan="5">Disc ${t.disc_number}</td></tr>`;
        }
        const dur = t.duration_seconds ? formatDuration(t.duration_seconds) : '--:--';
        html += `<tr class="tl-row" data-media-id="${t.id}" ondblclick="playTrackFromList(this)">`;
        html += `<td class="tl-num">${t.track_number || '-'}</td>`;
        html += `<td class="tl-title">${escapeHtml(t.title)}</td>`;
        html += `<td class="tl-artist">${escapeHtml(t.artist_name || '')}</td>`;
        html += `<td class="tl-duration">${dur}</td>`;
        html += `<td class="tl-play"><button class="btn-icon" onclick="playTrackFromList(this.closest('tr'))" title="Play">&#9654;</button></td>`;
        html += '</tr>';
    }
    html += '</tbody></table>';
    return html;
}

function formatDuration(seconds) {
    if (!seconds || seconds <= 0) return '--:--';
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    if (h > 0) return h + ':' + String(m).padStart(2, '0') + ':' + String(s).padStart(2, '0');
    return m + ':' + String(s).padStart(2, '0');
}

async function playAlbumTracks(albumId) {
    const data = await api('GET', '/albums/' + albumId + '/tracks');
    const tracks = (data.success && data.data) ? data.data : [];
    if (tracks.length === 0) return;
    const items = tracks.map(t => ({
        id: t.id, title: t.title, artist: t.artist_name || '',
        duration_seconds: t.duration_seconds || 0
    }));
    musicPlayer.queue = items;
    musicPlayer.currentIndex = 0;
    musicPlayer.playTrack();
}

async function shuffleAlbumTracks(albumId) {
    const data = await api('GET', '/albums/' + albumId + '/tracks');
    const tracks = (data.success && data.data) ? data.data : [];
    if (tracks.length === 0) return;
    const items = tracks.map(t => ({
        id: t.id, title: t.title, artist: t.artist_name || '',
        duration_seconds: t.duration_seconds || 0
    }));
    for (let i = items.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [items[i], items[j]] = [items[j], items[i]];
    }
    musicPlayer.queue = items;
    musicPlayer.currentIndex = 0;
    musicPlayer.playTrack();
}

async function playArtist(artistId) {
    const albumsData = await api('GET', '/artists/' + artistId + '/albums');
    const albums = (albumsData.success && albumsData.data) ? albumsData.data : [];
    const allTracks = [];
    for (const album of albums) {
        const td = await api('GET', '/albums/' + album.id + '/tracks');
        if (td.success && td.data) allTracks.push(...td.data);
    }
    if (allTracks.length === 0) return;
    musicPlayer.queue = allTracks.map(t => ({
        id: t.id, title: t.title, artist: t.artist_name || '',
        duration_seconds: t.duration_seconds || 0
    }));
    musicPlayer.currentIndex = 0;
    musicPlayer.playTrack();
}

async function shuffleArtist(artistId) {
    const albumsData = await api('GET', '/artists/' + artistId + '/albums');
    const albums = (albumsData.success && albumsData.data) ? albumsData.data : [];
    const allTracks = [];
    for (const album of albums) {
        const td = await api('GET', '/albums/' + album.id + '/tracks');
        if (td.success && td.data) allTracks.push(...td.data);
    }
    if (allTracks.length === 0) return;
    const items = allTracks.map(t => ({
        id: t.id, title: t.title, artist: t.artist_name || '',
        duration_seconds: t.duration_seconds || 0
    }));
    for (let i = items.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [items[i], items[j]] = [items[j], items[i]];
    }
    musicPlayer.queue = items;
    musicPlayer.currentIndex = 0;
    musicPlayer.playTrack();
}

function playTrackFromList(row) {
    const albumTracks = row.closest('.tracklist-table').querySelectorAll('.tl-row');
    const items = [];
    let startIndex = 0;
    albumTracks.forEach((tr, idx) => {
        const id = tr.dataset.mediaId;
        const title = tr.querySelector('.tl-title').textContent;
        const artist = tr.querySelector('.tl-artist').textContent;
        items.push({ id, title, artist, duration_seconds: 0 });
        if (tr === row) startIndex = idx;
    });
    musicPlayer.queue = items;
    musicPlayer.currentIndex = startIndex;
    musicPlayer.playTrack();
}

