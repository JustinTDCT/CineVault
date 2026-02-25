import { el, clear } from '../utils/dom.js';

export class VirtualGrid {
    constructor(container, opts = {}) {
        this.container = container;
        this.items = [];
        this.itemHeight = opts.itemHeight || 300;
        this.columns = opts.columns || 6;
        this.renderItem = opts.renderItem || (() => el('div'));
        this.buffer = 3;

        this.wrapper = el('div', { class: 'virtual-grid-wrapper', style: { position: 'relative', overflow: 'auto', height: '100%' } });
        this.inner = el('div', { class: 'virtual-grid-inner' });
        this.wrapper.appendChild(this.inner);
        container.appendChild(this.wrapper);

        this.wrapper.addEventListener('scroll', () => this.render());
        this.resizeObserver = new ResizeObserver(() => this.updateColumns());
        this.resizeObserver.observe(this.wrapper);
    }

    setItems(items) {
        this.items = items;
        this.updateColumns();
    }

    updateColumns() {
        const w = this.wrapper.clientWidth;
        if (w > 1200) this.columns = 8;
        else if (w > 900) this.columns = 6;
        else if (w > 600) this.columns = 4;
        else if (w > 400) this.columns = 3;
        else this.columns = 2;
        this.render();
    }

    render() {
        const totalRows = Math.ceil(this.items.length / this.columns);
        const totalHeight = totalRows * this.itemHeight;
        this.inner.style.height = totalHeight + 'px';

        const scrollTop = this.wrapper.scrollTop;
        const viewHeight = this.wrapper.clientHeight;
        const startRow = Math.max(0, Math.floor(scrollTop / this.itemHeight) - this.buffer);
        const endRow = Math.min(totalRows, Math.ceil((scrollTop + viewHeight) / this.itemHeight) + this.buffer);

        const startIdx = startRow * this.columns;
        const endIdx = Math.min(this.items.length, endRow * this.columns);

        clear(this.inner);

        const grid = el('div', {
            class: 'media-grid',
            style: {
                position: 'absolute',
                top: startRow * this.itemHeight + 'px',
                left: '0', right: '0',
            }
        });

        for (let i = startIdx; i < endIdx; i++) {
            grid.appendChild(this.renderItem(this.items[i], i));
        }

        this.inner.appendChild(grid);
    }

    destroy() {
        this.resizeObserver.disconnect();
    }
}
