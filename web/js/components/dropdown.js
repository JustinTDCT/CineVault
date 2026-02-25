import { el } from '../utils/dom.js';

export function select(options, value, onChange) {
    const sel = el('select', { class: 'form-select' });
    options.forEach(opt => {
        const option = el('option', { value: opt.value }, opt.label);
        if (opt.value === value) option.selected = true;
        sel.appendChild(option);
    });
    sel.addEventListener('change', () => onChange(sel.value));
    return sel;
}
