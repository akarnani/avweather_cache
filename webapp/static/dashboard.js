(() => {
    const searchInput = document.getElementById('searchInput');
    const searchForm = document.getElementById('searchForm');
    let searchTimeout;
    let currentPage = 1;
    let currentSearch = searchInput.value || '';

    function el(tag, opts) {
        const e = document.createElement(tag);
        if (!opts) return e;
        if (opts.text != null) e.textContent = opts.text;
        if (opts.className) e.className = opts.className;
        if (opts.dataset) Object.assign(e.dataset, opts.dataset);
        if (opts.style) Object.assign(e.style, opts.style);
        if (opts.attrs) for (const [k, v] of Object.entries(opts.attrs)) e.setAttribute(k, v);
        if (opts.children) opts.children.forEach(c => c && e.appendChild(c));
        return e;
    }

    function clear(node) {
        while (node.firstChild) node.removeChild(node.firstChild);
    }

    function formatTime(timeStr) {
        if (!timeStr) return 'Never';
        const date = new Date(timeStr);
        return date.toLocaleString('en-US', {
            year: 'numeric', month: '2-digit', day: '2-digit',
            hour: '2-digit', minute: '2-digit', second: '2-digit',
            timeZoneName: 'short',
        });
    }

    function formatAge(timeStr) {
        if (!timeStr) return 'N/A';
        const seconds = (Date.now() - new Date(timeStr).getTime()) / 1000;
        if (seconds < 60) return Math.floor(seconds) + 's ago';
        if (seconds < 3600) return Math.floor(seconds / 60) + 'm ago';
        if (seconds < 86400) return (seconds / 3600).toFixed(1) + 'h ago';
        return (seconds / 86400).toFixed(1) + 'd ago';
    }

    function formatTemp(temp) {
        return temp != null ? temp.toFixed(1) + '°C' : 'N/A';
    }

    function formatWind(dir, speed, gust) {
        if (!dir || speed == null) return 'N/A';
        let result = dir + '° @ ' + speed + 'kt';
        if (gust != null && gust > 0) result += ' G' + gust + 'kt';
        return result;
    }

    function updateTable(data) {
        const tbody = document.querySelector('#metarTable tbody');
        clear(tbody);

        data.metars.forEach(metar => {
            const fc = metar.flight_category || '';
            const row = el('tr', {
                children: [
                    el('td', { children: [el('strong', { text: metar.station_id || '' })] }),
                    el('td', { text: formatTime(metar.observation_time) }),
                    el('td', { text: formatAge(metar.observation_time) }),
                    el('td', { children: [el('span', { className: 'flight-cat ' + fc, text: fc })] }),
                    el('td', { text: formatTemp(metar.temp_c) }),
                    el('td', { text: formatWind(metar.wind_dir_degrees, metar.wind_speed_kt, metar.wind_gust_kt) }),
                    el('td', { text: metar.visibility_statute_mi || '' }),
                    el('td', { text: metar.raw_text || '', className: 'raw-text' }),
                ],
            });
            tbody.appendChild(row);
        });

        updatePagination(data);
        updateResultsInfo(data);
    }

    function updatePagination(data) {
        document.querySelectorAll('.pagination').forEach(p => {
            clear(p);
            p.appendChild(buildPagination(data));
        });
    }

    function buildPagination(data) {
        const frag = document.createDocumentFragment();
        const link = (text, page) => el('a', {
            text,
            attrs: { href: '#' },
            dataset: { action: 'load-page', page: String(page) },
        });
        const disabled = (text) => el('span', { className: 'disabled', text });

        if (data.page > 1) {
            frag.appendChild(link('« First', 1));
            frag.appendChild(link('‹ Prev', data.page - 1));
        } else {
            frag.appendChild(disabled('« First'));
            frag.appendChild(disabled('‹ Prev'));
        }
        frag.appendChild(el('span', { className: 'current', text: 'Page ' + data.page + ' of ' + data.total_pages }));
        if (data.page < data.total_pages) {
            frag.appendChild(link('Next ›', data.page + 1));
            frag.appendChild(link('Last »', data.total_pages));
        } else {
            frag.appendChild(disabled('Next ›'));
            frag.appendChild(disabled('Last »'));
        }
        return frag;
    }

    function updateResultsInfo(data) {
        const resultsInfo = document.querySelector('.results-info');
        if (data.search_query) {
            resultsInfo.textContent = 'Showing results ' + data.start_idx + '-' + data.end_idx + ' of ' + data.metar_count + ' (Page ' + data.page + ' of ' + data.total_pages + ')';
        } else {
            resultsInfo.textContent = 'Showing ' + data.start_idx + '-' + data.end_idx + ' of ' + data.metar_count + ' stations (Page ' + data.page + ' of ' + data.total_pages + ')';
        }

        const searchBox = document.querySelector('.search-box');
        let searchInfo = searchBox.querySelector('.search-info');

        if (data.search_query) {
            if (!searchInfo) {
                searchInfo = el('div', { className: 'search-info' });
                searchBox.appendChild(searchInfo);
            }
            clear(searchInfo);
            searchInfo.appendChild(document.createTextNode(
                'Showing ' + data.metar_count + ' of ' + data.total_count + ' stations matching "'
            ));
            searchInfo.appendChild(el('strong', { text: data.search_query }));
            searchInfo.appendChild(document.createTextNode('" '));
            searchInfo.appendChild(el('a', {
                text: 'Clear Search',
                attrs: { href: '#' },
                className: 'clear-link',
                dataset: { action: 'clear-search' },
            }));
        } else if (searchInfo) {
            searchInfo.remove();
        }
    }

    function loadPage(page) {
        currentPage = page;
        performSearch(currentSearch, page);
    }

    function clearSearch() {
        searchInput.value = '';
        currentSearch = '';
        currentPage = 1;
        performSearch('', 1);
        searchInput.focus();
    }

    function performSearch(query, page) {
        page = page || 1;
        const url = '/search?search=' + encodeURIComponent(query) + '&page=' + page;
        const newUrl = query ? '/?search=' + encodeURIComponent(query) + '&page=' + page : '/?page=' + page;
        window.history.pushState({ search: query, page }, '', newUrl);

        fetch(url)
            .then(response => response.json())
            .then(updateTable)
            .catch(error => console.error('Search error:', error));
    }

    // Delegated click handler for [data-action] elements (replaces inline onclick).
    document.addEventListener('click', (e) => {
        const target = e.target.closest('[data-action]');
        if (!target) return;
        e.preventDefault();
        const action = target.dataset.action;
        if (action === 'load-page') {
            const page = parseInt(target.dataset.page, 10);
            if (Number.isFinite(page) && page > 0) loadPage(page);
        } else if (action === 'clear-search') {
            clearSearch();
        }
    });

    // Suppress the form's default submit (we handle search via input/keypress).
    if (searchForm) {
        searchForm.addEventListener('submit', (e) => e.preventDefault());
    }

    searchInput.addEventListener('input', function () {
        clearTimeout(searchTimeout);
        const query = this.value.trim();
        if (query.length >= 2) {
            searchTimeout = setTimeout(() => {
                currentSearch = query;
                currentPage = 1;
                performSearch(query, 1);
            }, 500);
        } else if (query.length === 0) {
            clearSearch();
        }
    });

    searchInput.addEventListener('keypress', function (e) {
        if (e.key !== 'Enter') return;
        e.preventDefault();
        clearTimeout(searchTimeout);
        currentSearch = this.value.trim();
        currentPage = 1;
        if (currentSearch.length > 0) performSearch(currentSearch, 1);
    });

    window.addEventListener('popstate', function (e) {
        if (!e.state) return;
        currentSearch = e.state.search || '';
        currentPage = e.state.page || 1;
        searchInput.value = currentSearch;
        performSearch(currentSearch, currentPage);
    });

    searchInput.focus();

    setInterval(() => performSearch(currentSearch, currentPage), 30000);
})();
