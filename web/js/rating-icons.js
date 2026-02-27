// Dynamic rating icons — returns inline SVGs that change based on score thresholds.
// Two sizes: 'small' (14px, for poster overlays) and 'large' (20px, for detail page).

function ratingIconSize(size) {
    return size === 'large' ? 20 : 14;
}

function ratingIconIMDb(score, size) {
    const s = ratingIconSize(size);
    let fill, textFill;
    if (score >= 7.0) { fill = '#f5c518'; textFill = '#000'; }
    else if (score >= 5.0) { fill = '#c9a010'; textFill = '#1a1a1a'; }
    else { fill = '#8a7420'; textFill = '#333'; }
    return `<svg width="${s}" height="${s}" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
        <rect rx="4" width="24" height="24" fill="${fill}"/>
        <text x="12" y="16.5" text-anchor="middle" font-family="Arial,sans-serif" font-weight="900" font-size="10" fill="${textFill}" letter-spacing="-0.5">IMDb</text>
    </svg>`;
}

function ratingIconRTCritic(score, size) {
    const s = ratingIconSize(size);
    if (score >= 60) {
        // Fresh tomato
        return `<svg width="${s}" height="${s}" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
            <path d="M12 3c-.4 0-.8.1-1.1.2C10 2.5 9 2 8 2c.3.8.4 1.5.2 2.2C6.8 4.8 5.8 5.9 5.2 7.2 4.4 8.8 4 10.6 4 12.5 4 17.2 7.6 21 12 21s8-3.8 8-8.5c0-1.9-.4-3.7-1.2-5.3C17.5 5 15 3.5 12 3z" fill="#FA320A"/>
            <path d="M13.5 2c-.5 0-1 .3-1.2.7C11.5 1.5 10.2 1 9 1c.5 1 .6 2 .3 3" fill="#00912D" stroke="#00912D" stroke-width="0.5"/>
            <path d="M10 9c-1 .5-1.8 1.5-2 2.8-.1.8.1 1.5.5 2" fill="none" stroke="rgba(255,255,255,0.3)" stroke-width="1.2" stroke-linecap="round"/>
        </svg>`;
    }
    // Rotten splat
    return `<svg width="${s}" height="${s}" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
        <path d="M12 4C9.5 4 7.5 5 6 7c-1 1.3-1.5 3-1.5 4.8 0 2 .7 3.8 2 5.2 1.5 1.6 3.5 2.5 5.5 2.5s4-1 5.5-2.5c1.3-1.4 2-3.2 2-5.2 0-1.8-.5-3.5-1.5-4.8C16.5 5 14.5 4 12 4z" fill="#0BAD1A"/>
        <circle cx="8" cy="10" r="1.8" fill="#0BAD1A" stroke="#087A12" stroke-width="0.3"/>
        <circle cx="16" cy="8" r="1.5" fill="#0BAD1A" stroke="#087A12" stroke-width="0.3"/>
        <circle cx="17" cy="15" r="1.3" fill="#0BAD1A" stroke="#087A12" stroke-width="0.3"/>
        <circle cx="6" cy="14" r="1.2" fill="#0BAD1A" stroke="#087A12" stroke-width="0.3"/>
        <circle cx="10" cy="10" r="1.2" fill="#065F0E"/>
        <circle cx="14" cy="11" r="1" fill="#065F0E"/>
        <path d="M9.5 14c.5 1 1.5 1.5 2.5 1.5s2-.5 2.5-1.5" fill="none" stroke="#065F0E" stroke-width="1" stroke-linecap="round"/>
    </svg>`;
}

function ratingIconRTAudience(score, size) {
    const s = ratingIconSize(size);
    if (score >= 60) {
        // Full upright popcorn
        return `<svg width="${s}" height="${s}" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
            <path d="M7 10l1.5 11h7L17 10H7z" fill="#FA320A"/>
            <path d="M8 10l.3 2h7.4l.3-2H8z" fill="#D42A08" opacity="0.5"/>
            <path d="M7 10h10" stroke="#FFF" stroke-width="0.5" opacity="0.3"/>
            <circle cx="10" cy="6" r="2" fill="#FEDE6B"/>
            <circle cx="14" cy="6" r="2" fill="#FEDE6B"/>
            <circle cx="12" cy="4.5" r="2" fill="#FEDE6B"/>
            <circle cx="8.5" cy="7.5" r="1.8" fill="#FEDE6B"/>
            <circle cx="15.5" cy="7.5" r="1.8" fill="#FEDE6B"/>
            <circle cx="12" cy="7" r="1.5" fill="#F5D85A"/>
        </svg>`;
    }
    // Spilled/tipped popcorn
    return `<svg width="${s}" height="${s}" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
        <g transform="rotate(30, 12, 14)">
            <path d="M7 10l1.5 11h7L17 10H7z" fill="#6BC53A"/>
            <path d="M8 10l.3 2h7.4l.3-2H8z" fill="#5AA832" opacity="0.5"/>
        </g>
        <circle cx="6" cy="8" r="1.5" fill="#D4CF6B"/>
        <circle cx="17" cy="5" r="1.3" fill="#D4CF6B"/>
        <circle cx="8" cy="5" r="1.2" fill="#D4CF6B"/>
        <circle cx="14" cy="3" r="1" fill="#D4CF6B"/>
        <circle cx="19" cy="8" r="1.1" fill="#D4CF6B"/>
    </svg>`;
}

function ratingIconTMDB(score, size) {
    const s = ratingIconSize(size);
    let ringColor;
    if (score >= 7.0) ringColor = '#21d07a';
    else if (score >= 5.0) ringColor = '#d2d531';
    else ringColor = '#db2360';
    const pct = (score / 10) * 100;
    const dashLen = 2 * Math.PI * 8.5;
    const filled = dashLen * (pct / 100);
    return `<svg width="${s}" height="${s}" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
        <circle cx="12" cy="12" r="11" fill="#081C22"/>
        <circle cx="12" cy="12" r="8.5" fill="none" stroke="#1f3a3f" stroke-width="2.5"/>
        <circle cx="12" cy="12" r="8.5" fill="none" stroke="${ringColor}" stroke-width="2.5"
            stroke-dasharray="${filled} ${dashLen - filled}"
            stroke-linecap="round" transform="rotate(-90 12 12)"/>
    </svg>`;
}

function ratingIconMetacritic(score, size) {
    const s = ratingIconSize(size);
    let bg;
    if (score >= 61) bg = '#6c3';
    else if (score >= 40) bg = '#fc3';
    else bg = '#f00';
    return `<svg width="${s}" height="${s}" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
        <rect rx="3" width="24" height="24" fill="${bg}"/>
        <text x="12" y="17" text-anchor="middle" font-family="Arial,sans-serif" font-weight="900" font-size="14" fill="#fff">${score}</text>
    </svg>`;
}

/**
 * Returns the appropriate rating icon SVG for overlay badges.
 * @param {'imdb'|'rt_critic'|'rt_audience'|'tmdb'|'metacritic'} source
 * @param {number} score - raw score value
 * @param {'small'|'large'} [size='small']
 */
function ratingIcon(source, score, size) {
    size = size || 'small';
    switch (source) {
        case 'imdb': return ratingIconIMDb(score, size);
        case 'rt_critic': return ratingIconRTCritic(score, size);
        case 'rt_audience': return ratingIconRTAudience(score, size);
        case 'tmdb': return ratingIconTMDB(score, size);
        case 'metacritic': return ratingIconMetacritic(score, size);
        default: return '';
    }
}
