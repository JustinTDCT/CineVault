const API_BASE = '/api';

let authToken = null;

export function setToken(token) { authToken = token; }
export function getToken() { return authToken; }

async function request(method, path, body = null) {
    const opts = {
        method,
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
    };

    if (authToken) {
        opts.headers['Authorization'] = `Bearer ${authToken}`;
    }

    if (body) {
        opts.body = JSON.stringify(body);
    }

    const resp = await fetch(`${API_BASE}${path}`, opts);
    const data = await resp.json().catch(() => null);

    if (!resp.ok) {
        const msg = data?.error?.message || resp.statusText;
        throw new ApiError(resp.status, data?.error?.code || 'UNKNOWN', msg);
    }

    if (data && typeof data === 'object' && 'data' in data) {
        return data.data ?? [];
    }
    return data;
}

export class ApiError extends Error {
    constructor(status, code, message) {
        super(message);
        this.status = status;
        this.code = code;
    }
}

export const get = (path) => request('GET', path);
export const post = (path, body) => request('POST', path, body);
export const put = (path, body) => request('PUT', path, body);
export const patch = (path, body) => request('PATCH', path, body);
export const del = (path) => request('DELETE', path);

export const auth = {
    register: (data) => post('/auth/register', data),
    login: (data) => post('/auth/login', data),
    loginPIN: (data) => post('/auth/login/pin', data),
    logout: () => post('/auth/logout'),
};

export const users = {
    me: () => get('/users/me'),
    profile: () => get('/users/me/profile'),
    updateProfile: (data) => put('/users/me/profile', data),
    list: () => get('/users'),
    pinUsers: () => get('/users/pin-users'),
};

export const libraries = {
    list: () => get('/libraries'),
    get: (id) => get(`/libraries/${id}`),
    create: (data) => post('/libraries', data),
    update: (id, data) => put(`/libraries/${id}`, data),
    del: (id) => del(`/libraries/${id}`),
    types: () => get('/libraries/types'),
    browse: (path) => get(`/libraries/browse?path=${encodeURIComponent(path || '/')}`),
    permissions: (id) => get(`/libraries/${id}/permissions`),
    setPermissions: (id, data) => put(`/libraries/${id}/permissions`, data),
};

export const media = {
    list: (libId, params = {}) => {
        const q = new URLSearchParams({ library_id: libId, ...params });
        return get(`/media?${q}`);
    },
    get: (id) => get(`/media/${id}`),
    children: (id) => get(`/media/${id}/children`),
};

export const scanner = {
    scan: (libId) => post(`/scanner/library/${libId}`),
    status: (libId) => get(`/scanner/status/${libId}`),
};

export const metadata = {
    search: (title, type, year) => {
        const q = new URLSearchParams({ title, type });
        if (year) q.set('year', year);
        return get(`/metadata/search?${q}`);
    },
    match: (itemId, cacheId) => post(`/metadata/match/${itemId}`, { cache_id: cacheId }),
    refresh: (itemId) => post(`/metadata/refresh/${itemId}`),
};

export const settings = {
    list: () => get('/settings'),
    update: (data) => put('/settings', data),
};
