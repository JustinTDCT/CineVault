import { el, clear, qs } from '../utils/dom.js';

let currentModal = null;

export function open(title, contentFn, opts = {}) {
    close();

    const overlay = el('div', { class: 'modal-overlay' });
    const modal = el('div', { class: 'modal' });

    const header = el('div', { class: 'modal-header' },
        el('h2', { class: 'modal-title' }, title),
        el('button', { class: 'modal-close', onClick: close }, '\u00d7')
    );

    const body = el('div', { class: 'modal-body' });
    contentFn(body);

    modal.appendChild(header);
    modal.appendChild(body);

    if (opts.footer) {
        const footer = el('div', { class: 'modal-footer' });
        opts.footer(footer);
        modal.appendChild(footer);
    }

    overlay.appendChild(modal);
    overlay.addEventListener('click', (e) => {
        if (e.target === overlay) close();
    });

    qs('#modal-root').appendChild(overlay);
    currentModal = overlay;
    document.body.style.overflow = 'hidden';
}

export function close() {
    if (currentModal) {
        currentModal.remove();
        currentModal = null;
        document.body.style.overflow = '';
    }
}

export function confirm(message, onConfirm) {
    open('Confirm', (body) => {
        body.appendChild(el('p', {}, message));
    }, {
        footer: (footer) => {
            footer.appendChild(el('button', { class: 'btn btn-secondary', onClick: close }, 'Cancel'));
            footer.appendChild(el('button', { class: 'btn btn-primary', onClick: () => { close(); onConfirm(); } }, 'Confirm'));
        }
    });
}
