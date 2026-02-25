import { el, clear } from '../utils/dom.js';

export function tabs(items, container) {
    const bar = el('div', { class: 'tabs' });
    const body = el('div', { class: 'tab-content' });

    items.forEach((item, i) => {
        const btn = el('button', {
            class: `tab${i === 0 ? ' active' : ''}`,
            onClick: () => {
                bar.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
                btn.classList.add('active');
                clear(body);
                item.render(body);
            },
        }, item.label);
        bar.appendChild(btn);
    });

    container.appendChild(bar);
    container.appendChild(body);

    if (items.length > 0) items[0].render(body);
}
