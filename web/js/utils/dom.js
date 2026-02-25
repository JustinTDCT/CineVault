export function el(tag, attrs = {}, ...children) {
    const elem = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) {
        if (k === 'class') elem.className = v;
        else if (k === 'style' && typeof v === 'object') Object.assign(elem.style, v);
        else if (k.startsWith('on')) elem.addEventListener(k.slice(2).toLowerCase(), v);
        else if (k === 'html') elem.innerHTML = v;
        else elem.setAttribute(k, v);
    }
    for (const child of children) {
        if (typeof child === 'string') elem.appendChild(document.createTextNode(child));
        else if (child) elem.appendChild(child);
    }
    return elem;
}

export function clear(container) {
    container.innerHTML = '';
}

export function show(elem) { elem.style.display = ''; }
export function hide(elem) { elem.style.display = 'none'; }

export function qs(selector, root = document) { return root.querySelector(selector); }
export function qsa(selector, root = document) { return [...root.querySelectorAll(selector)]; }

export function onDelegate(parent, event, selector, handler) {
    parent.addEventListener(event, (e) => {
        const target = e.target.closest(selector);
        if (target && parent.contains(target)) handler(e, target);
    });
}
