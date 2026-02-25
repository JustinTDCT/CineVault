import { el, clear, qs } from '../utils/dom.js';
import * as api from '../services/api.js';
import * as toast from '../components/toast.js';

export async function init(container, onAuth) {
    clear(container);

    const hasUsers = await checkHasUsers();
    if (hasUsers) {
        renderLogin(container, onAuth);
    } else {
        renderRegister(container, onAuth);
    }
}

function renderLogin(container, onAuth) {
    const form = el('div', { class: 'card', style: { maxWidth: '400px', margin: '80px auto' } });
    form.appendChild(el('h2', { style: { marginBottom: '20px' } }, 'Sign In'));

    form.appendChild(formGroup('Email', 'email', 'email'));
    form.appendChild(formGroup('Password', 'password', 'password'));

    const btn = el('button', { class: 'btn btn-primary', style: { width: '100%', marginTop: '8px' } }, 'Sign In');
    btn.addEventListener('click', async () => {
        const email = qs('#field-email', form).value;
        const password = qs('#field-password', form).value;
        try {
            const data = await api.auth.login({ email, password });
            api.setToken(data.token);
            onAuth(data);
        } catch (e) {
            toast.error(e.message);
        }
    });
    form.appendChild(btn);

    const switchLink = el('div', { style: { marginTop: '16px', textAlign: 'center', fontSize: '13px' } });
    switchLink.appendChild(el('a', {
        href: '#',
        style: { color: 'var(--accent)' },
        onClick: (e) => { e.preventDefault(); renderRegister(container, onAuth); },
    }, 'Create Account'));
    form.appendChild(switchLink);

    container.appendChild(form);
}

function renderRegister(container, onAuth) {
    clear(container);
    const form = el('div', { class: 'card', style: { maxWidth: '400px', margin: '80px auto' } });
    form.appendChild(el('h2', { style: { marginBottom: '20px' } }, 'Create Account'));

    form.appendChild(formGroup('Full Name', 'text', 'fullname'));
    form.appendChild(formGroup('Email', 'email', 'email'));
    form.appendChild(formGroup('Password', 'password', 'password'));
    form.appendChild(formGroup('PIN (optional)', 'text', 'pin'));

    const btn = el('button', { class: 'btn btn-primary', style: { width: '100%', marginTop: '8px' } }, 'Create Account');
    btn.addEventListener('click', async () => {
        const full_name = qs('#field-fullname', form).value;
        const email = qs('#field-email', form).value;
        const password = qs('#field-password', form).value;
        const pin = qs('#field-pin', form).value;
        try {
            const data = await api.auth.register({ full_name, email, password, pin: pin || undefined });
            api.setToken(data.token);
            onAuth(data);
        } catch (e) {
            toast.error(e.message);
        }
    });
    form.appendChild(btn);
    container.appendChild(form);
}

function formGroup(label, type, id) {
    return el('div', { class: 'form-group' },
        el('label', { class: 'form-label', for: `field-${id}` }, label),
        el('input', { class: 'form-input', type, id: `field-${id}`, autocomplete: type === 'password' ? 'current-password' : 'on' })
    );
}

async function checkHasUsers() {
    try {
        await api.get('/health');
        const resp = await fetch('/api/users/pin-users', { credentials: 'include' });
        return resp.status !== 401;
    } catch {
        return false;
    }
}
