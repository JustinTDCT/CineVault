import { el } from '../utils/dom.js';

export function toggle(label, checked, onChange) {
    const row = el('div', { class: 'toggle-row' });
    const lbl = el('span', { class: 'toggle-label' }, label);

    const wrapper = el('label', { class: 'toggle' });
    const input = el('input', { type: 'checkbox' });
    input.checked = checked;
    input.addEventListener('change', () => onChange(input.checked));
    const slider = el('span', { class: 'toggle-slider' });

    wrapper.appendChild(input);
    wrapper.appendChild(slider);
    row.appendChild(lbl);
    row.appendChild(wrapper);
    return row;
}
