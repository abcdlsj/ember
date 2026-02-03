// Ember Web UI Application
const app = {
    // State
    currentSection: 'resume',
    currentItems: [],
    currentPage: 0,
    totalItems: 0,
    pageSize: 20,
    navStack: [],
    currentServer: null,
    servers: [],
    editingServerIndex: -1,
    currentItem: null,
    searchQuery: '',
    isSearchOpen: false,
    
    // DOM Elements
    elements: {},
    
    // Initialization
    init() {
        this.cacheElements();
        this.loadTheme();
        this.fetchStatus();
        this.fetchServers();
        this.loadSection('resume');
        
        // Set up periodic status updates
        setInterval(() => this.fetchStatus(), 30000);
    },
    
    cacheElements() {
        this.elements = {
            mediaGrid: document.getElementById('media-grid'),
            loadingState: document.getElementById('loading-state'),
            emptyState: document.getElementById('empty-state'),
            pagination: document.getElementById('pagination'),
            paginationInfo: document.getElementById('pagination-info'),
            prevBtn: document.getElementById('prev-btn'),
            nextBtn: document.getElementById('next-btn'),
            breadcrumb: document.getElementById('breadcrumb'),
            breadcrumbCurrent: document.getElementById('breadcrumb-current'),
            searchBar: document.getElementById('search-bar'),
            searchInput: document.getElementById('search-input'),
            serverModal: document.getElementById('server-modal'),
            serverList: document.getElementById('server-list'),
            serverForm: document.getElementById('server-form'),
            serverModalFooter: document.getElementById('server-modal-footer'),
            playerModal: document.getElementById('player-modal'),
            videoPlayer: document.getElementById('video-player'),
            playerTitle: document.getElementById('player-title'),
            playerInfo: document.getElementById('player-info'),
            playerSubtitles: document.getElementById('player-subtitles'),
            playerFavBtn: document.getElementById('player-fav-btn'),
            serverName: document.getElementById('server-name'),
            serverStatus: document.getElementById('server-status'),
            latency: document.getElementById('latency'),
            mpvText: document.getElementById('mpv-text'),
            toastContainer: document.getElementById('toast-container'),
        };
    },
    
    // Theme Management
    loadTheme() {
        const theme = localStorage.getItem('theme') || 'auto';
        this.applyTheme(theme);
    },
    
    toggleTheme() {
        const current = document.documentElement.getAttribute('data-theme');
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        let newTheme;
        
        if (current === 'light') {
            newTheme = 'dark';
        } else if (current === 'dark') {
            newTheme = 'light';
        } else {
            newTheme = prefersDark ? 'light' : 'dark';
        }
        
        this.applyTheme(newTheme);
        localStorage.setItem('theme', newTheme);
    },
    
    applyTheme(theme) {
        if (theme === 'auto') {
            document.documentElement.removeAttribute('data-theme');
        } else {
            document.documentElement.setAttribute('data-theme', theme);
        }
    },
    
    // Navigation
    switchSection(section) {
        this.currentSection = section;
        this.currentPage = 0;
        this.navStack = [];
        this.searchQuery = '';
        
        // Update nav active state
        document.querySelectorAll('.navbar-nav a').forEach(el => {
            el.classList.toggle('active', el.dataset.section === section);
        });
        
        this.hideSearch();
        this.loadSection(section);
    },
    
    async loadSection(section, parentId = '') {
        this.showLoading();
        this.hideBreadcrumb();
        
        try {
            let endpoint;
            switch (section) {
                case 'resume':
                    endpoint = '/api/resume';
                    break;
                case 'favorites':
                    endpoint = '/api/favorites';
                    break;
                case 'libraries':
                    endpoint = '/api/libraries';
                    break;
                case 'search':
                    endpoint = `/api/search?q=${encodeURIComponent(this.searchQuery)}`;
                    break;
                default:
                    endpoint = `/api/items?parentId=${parentId}&page=${this.currentPage}`;
            }
            
            const response = await fetch(endpoint);
            const data = await response.json();
            
            if (response.ok) {
                this.currentItems = data.items || [];
                this.totalItems = data.total || this.currentItems.length;
                this.renderItems();
                this.renderPagination();
            } else {
                this.showError(data.error || 'Failed to load items');
                this.showEmpty();
            }
        } catch (error) {
            console.error('Load section error:', error);
            this.showError('Failed to connect to server');
            this.showEmpty();
        }
    },
    
    // Item Rendering
    renderItems() {
        if (this.currentItems.length === 0) {
            this.showEmpty();
            return;
        }
        
        this.hideLoading();
        this.hideEmpty();
        
        const grid = this.elements.mediaGrid;
        grid.innerHTML = this.currentItems.map(item => this.createMediaCard(item)).join('');
    },
    
    createMediaCard(item) {
        const typeLabels = {
            'Movie': 'Movie',
            'Series': 'Series',
            'Season': 'Season',
            'Episode': 'EP',
            'CollectionFolder': 'Library',
            'Folder': 'Folder',
            'BoxSet': 'Box Set'
        };
        
        const isPlayable = ['Movie', 'Episode', 'Video'].includes(item.type);
        const hasProgress = item.userData?.playbackPositionTicks > 0;
        const progressPercent = hasProgress 
            ? Math.round((item.userData.playbackPositionTicks / item.runTimeTicks) * 100)
            : 0;
        
        const year = item.year ? `(${item.year})` : '';
        const episodeInfo = item.indexNumber ? `EP ${item.indexNumber}` : '';
        const seriesInfo = item.seriesName ? `${item.seriesName} ${year}` : '';
        
        let title = item.name;
        if (item.type === 'Episode' && item.seriesName) {
            title = `${item.seriesName} - ${item.name}`;
        }
        
        return `
            <div class="media-card animate-fade-in" onclick="app.selectItem('${item.id}')">
                <div class="media-poster">
                    ${item.imageUrl 
                        ? `<img src="${item.imageUrl}" alt="${item.name}" loading="lazy">`
                        : `<div class="media-poster-placeholder">${typeLabels[item.type] || item.type}</div>`
                    }
                    ${hasProgress ? `
                        <div class="media-progress">
                            <div class="media-progress-bar" style="width: ${progressPercent}%"></div>
                        </div>
                    ` : ''}
                    ${item.userData?.isFavorite ? `
                        <div class="media-favorite">
                            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
                                <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"></polygon>
                            </svg>
                        </div>
                    ` : ''}
                    ${isPlayable ? `
                        <div class="media-actions" onclick="event.stopPropagation()">
                            <button class="media-action-btn" onclick="app.playItem('${item.id}')" title="Play">
                                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                    <polygon points="5 3 19 12 5 21 5 3"></polygon>
                                </svg>
                            </button>
                        </div>
                    ` : ''}
                </div>
                <div class="media-info">
                    <div class="media-title">${title}</div>
                    <div class="media-meta">
                        <span>${typeLabels[item.type] || item.type}</span>
                        ${item.year ? `<span class="media-meta-dot"></span><span>${item.year}</span>` : ''}
                        ${item.productionYear ? `<span class="media-meta-dot"></span><span>${item.productionYear}</span>` : ''}
                    </div>
                </div>
            </div>
        `;
    },
    
    // Item Selection
    async selectItem(itemId) {
        const item = this.currentItems.find(i => i.id === itemId);
        if (!item) return;
        
        switch (item.type) {
            case 'Movie':
            case 'Episode':
            case 'Video':
                this.playItem(itemId);
                break;
            case 'Series':
                this.pushNav(item);
                await this.loadSeasons(itemId);
                break;
            case 'Season':
                this.pushNav(item);
                await this.loadEpisodes(item.seriesId || item.parentId, itemId);
                break;
            case 'CollectionFolder':
            case 'Folder':
            case 'BoxSet':
                this.pushNav(item);
                await this.loadFolder(itemId);
                break;
        }
    },
    
    async loadSeasons(seriesId) {
        this.showLoading();
        try {
            const response = await fetch(`/api/seasons?seriesId=${seriesId}`);
            const data = await response.json();
            
            if (response.ok) {
                this.currentItems = data.items || [];
                this.renderItems();
                this.showBreadcrumb('Seasons');
            }
        } catch (error) {
            this.showError('Failed to load seasons');
        }
    },
    
    async loadEpisodes(seriesId, seasonId) {
        this.showLoading();
        try {
            const response = await fetch(`/api/episodes?seriesId=${seriesId}&seasonId=${seasonId}`);
            const data = await response.json();
            
            if (response.ok) {
                this.currentItems = data.items || [];
                this.renderItems();
                this.showBreadcrumb('Episodes');
            }
        } catch (error) {
            this.showError('Failed to load episodes');
        }
    },
    
    async loadFolder(parentId) {
        this.showLoading();
        try {
            const response = await fetch(`/api/items?parentId=${parentId}&page=${this.currentPage}`);
            const data = await response.json();
            
            if (response.ok) {
                this.currentItems = data.items || [];
                this.totalItems = data.total || this.currentItems.length;
                this.renderItems();
                this.renderPagination();
                this.showBreadcrumb('Items');
            }
        } catch (error) {
            this.showError('Failed to load items');
        }
    },
    
    // Navigation Stack
    pushNav(item) {
        this.navStack.push({
            items: [...this.currentItems],
            total: this.totalItems,
            page: this.currentPage,
            title: item.name
        });
    },
    
    goBack() {
        if (this.navStack.length === 0) return;
        
        const prev = this.navStack.pop();
        this.currentItems = prev.items;
        this.totalItems = prev.total;
        this.currentPage = prev.page;
        this.renderItems();
        this.renderPagination();
        
        if (this.navStack.length === 0) {
            this.hideBreadcrumb();
        } else {
            this.showBreadcrumb(this.navStack[this.navStack.length - 1].title);
        }
    },
    
    goHome() {
        this.navStack = [];
        this.switchSection('resume');
    },
    
    // Pagination
    renderPagination() {
        const totalPages = Math.ceil(this.totalItems / this.pageSize);
        
        if (totalPages <= 1) {
            this.elements.pagination.style.display = 'none';
            return;
        }
        
        this.elements.pagination.style.display = 'flex';
        this.elements.paginationInfo.textContent = `Page ${this.currentPage + 1} of ${totalPages}`;
        this.elements.prevBtn.disabled = this.currentPage === 0;
        this.elements.nextBtn.disabled = this.currentPage >= totalPages - 1;
    },
    
    prevPage() {
        if (this.currentPage > 0) {
            this.currentPage--;
            this.loadCurrentView();
        }
    },
    
    nextPage() {
        const totalPages = Math.ceil(this.totalItems / this.pageSize);
        if (this.currentPage < totalPages - 1) {
            this.currentPage++;
            this.loadCurrentView();
        }
    },
    
    loadCurrentView() {
        if (this.navStack.length > 0) {
            const current = this.navStack[this.navStack.length - 1];
            this.loadFolder(current.items[0]?.parentId || '');
        } else {
            this.loadSection(this.currentSection);
        }
    },
    
    // Search
    toggleSearch() {
        this.isSearchOpen = !this.isSearchOpen;
        this.elements.searchBar.style.display = this.isSearchOpen ? 'block' : 'none';
        if (this.isSearchOpen) {
            setTimeout(() => this.elements.searchInput.focus(), 100);
        }
    },
    
    hideSearch() {
        this.isSearchOpen = false;
        this.elements.searchBar.style.display = 'none';
        this.elements.searchInput.value = '';
    },
    
    handleSearch(event) {
        if (event.key === 'Enter') {
            this.searchQuery = this.elements.searchInput.value.trim();
            if (this.searchQuery) {
                this.currentPage = 0;
                this.loadSection('search');
            }
        } else if (event.key === 'Escape') {
            this.toggleSearch();
        }
    },
    
    // Video Playback
    async playItem(itemId) {
        try {
            const response = await fetch(`/api/stream?itemId=${itemId}`);
            const data = await response.json();
            
            if (response.ok) {
                this.currentItem = data;
                this.openPlayer(data);
            } else {
                this.showError(data.error || 'Failed to load stream');
            }
        } catch (error) {
            this.showError('Failed to start playback');
        }
    },
    
    openPlayer(data) {
        this.elements.playerTitle.textContent = data.name || 'Now Playing';
        this.elements.playerModal.style.display = 'flex';
        
        const video = this.elements.videoPlayer;
        video.src = data.streamUrl;
        video.poster = data.posterUrl || '';
        
        // Add subtitles
        this.elements.playerSubtitles.innerHTML = '';
        if (data.subtitles && data.subtitles.length > 0) {
            data.subtitles.forEach(sub => {
                const track = document.createElement('track');
                track.kind = 'subtitles';
                track.src = sub.url;
                track.srclang = sub.language || 'und';
                track.label = sub.title || sub.language || 'Subtitle';
                if (sub.isDefault) track.default = true;
                video.appendChild(track);
                
                const tag = document.createElement('span');
                tag.className = 'subtitle-tag';
                tag.textContent = sub.language || 'Subtitle';
                this.elements.playerSubtitles.appendChild(tag);
            });
        }
        
        // Update favorite button
        this.updatePlayerFavButton(data.isFavorite);
        
        // Update info
        const info = [];
        if (data.container) info.push(data.container.toUpperCase());
        if (data.duration) info.push(this.formatDuration(data.duration));
        this.elements.playerInfo.textContent = info.join(' • ');
        
        // Start playback
        video.play().catch(() => {});
        
        // Report playback start
        this.reportPlayback('start', data.itemId, 0);
        
        // Set up progress reporting
        this.playbackInterval = setInterval(() => {
            if (!video.paused) {
                this.reportPlayback('progress', data.itemId, Math.floor(video.currentTime * 10000000));
            }
        }, 30000);
        
        // Handle video end
        video.onended = () => {
            this.reportPlayback('stop', data.itemId, Math.floor(video.duration * 10000000));
        };
    },
    
    closePlayer() {
        const video = this.elements.videoPlayer;
        
        // Report stop
        if (this.currentItem && video.currentTime > 0) {
            this.reportPlayback('stop', this.currentItem.itemId, Math.floor(video.currentTime * 10000000));
        }
        
        // Clear interval
        if (this.playbackInterval) {
            clearInterval(this.playbackInterval);
            this.playbackInterval = null;
        }
        
        // Stop video
        video.pause();
        video.src = '';
        video.innerHTML = '';
        
        this.elements.playerModal.style.display = 'none';
        this.currentItem = null;
    },
    
    async reportPlayback(type, itemId, positionTicks) {
        try {
            await fetch('/api/playback', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ type, itemId, positionTicks })
            });
        } catch (error) {
            console.error('Playback report error:', error);
        }
    },
    
    async toggleFavoriteCurrent() {
        if (!this.currentItem) return;
        
        try {
            const response = await fetch(`/api/favorite?itemId=${this.currentItem.itemId}`, {
                method: 'POST'
            });
            const data = await response.json();
            
            if (response.ok) {
                this.updatePlayerFavButton(data.isFavorite);
                this.showToast(data.isFavorite ? 'Added to favorites' : 'Removed from favorites', 'success');
                
                // Update item in current items
                const item = this.currentItems.find(i => i.id === this.currentItem.itemId);
                if (item && item.userData) {
                    item.userData.isFavorite = data.isFavorite;
                    this.renderItems();
                }
            }
        } catch (error) {
            this.showError('Failed to toggle favorite');
        }
    },
    
    updatePlayerFavButton(isFavorite) {
        const btn = this.elements.playerFavBtn;
        if (isFavorite) {
            btn.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"></polygon></svg>`;
            btn.style.color = 'var(--star-color)';
        } else {
            btn.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"></polygon></svg>`;
            btn.style.color = '';
        }
    },
    
    // Server Management
    openServerModal() {
        this.elements.serverModal.style.display = 'flex';
        this.renderServerList();
    },
    
    closeServerModal() {
        this.elements.serverModal.style.display = 'none';
        this.cancelServerEdit();
    },
    
    renderServerList() {
        const list = this.elements.serverList;
        
        if (this.servers.length === 0) {
            list.innerHTML = '<p class="text-muted" style="text-align: center; padding: 2rem;">No servers configured</p>';
            return;
        }
        
        list.innerHTML = this.servers.map((server, index) => `
            <div class="server-item ${server.isActive ? 'active' : ''}" onclick="app.selectServer(${index})">
                <div class="server-item-info">
                    <div>
                        <div class="server-item-name">${server.name || server.url}</div>
                        <div class="server-item-url">${server.url}</div>
                    </div>
                </div>
                <div style="display: flex; align-items: center; gap: 0.5rem;">
                    ${server.latency !== undefined ? `
                        <span class="server-item-latency ${this.getLatencyClass(server.latency)}">
                            ${server.latency}ms
                        </span>
                    ` : ''}
                    <div class="server-item-actions" onclick="event.stopPropagation()">
                        <button class="btn btn-ghost btn-icon btn-sm" onclick="app.editServer(${index})" title="Edit">
                            <svg class="icon-sm" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
                                <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                            </svg>
                        </button>
                        <button class="btn btn-ghost btn-icon btn-sm" onclick="app.deleteServer(${index})" title="Delete">
                            <svg class="icon-sm" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <polyline points="3 6 5 6 21 6"></polyline>
                                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                            </svg>
                        </button>
                    </div>
                </div>
            </div>
        `).join('');
    },
    
    getLatencyClass(latency) {
        if (latency < 100) return 'good';
        if (latency < 500) return 'medium';
        return 'bad';
    },
    
    showServerForm() {
        this.editingServerIndex = -1;
        document.getElementById('server-form-title').textContent = 'Add Server';
        document.getElementById('server-name-input').value = '';
        document.getElementById('server-url-input').value = '';
        document.getElementById('server-username-input').value = '';
        document.getElementById('server-password-input').value = '';
        
        this.elements.serverList.style.display = 'none';
        this.elements.serverModalFooter.style.display = 'none';
        this.elements.serverForm.style.display = 'block';
    },
    
    editServer(index) {
        this.editingServerIndex = index;
        const server = this.servers[index];
        
        document.getElementById('server-form-title').textContent = 'Edit Server';
        document.getElementById('server-name-input').value = server.name || '';
        document.getElementById('server-url-input').value = server.url || '';
        document.getElementById('server-username-input').value = server.username || '';
        document.getElementById('server-password-input').value = server.password || '';
        
        this.elements.serverList.style.display = 'none';
        this.elements.serverModalFooter.style.display = 'none';
        this.elements.serverForm.style.display = 'block';
    },
    
    cancelServerEdit() {
        this.editingServerIndex = -1;
        this.elements.serverForm.style.display = 'none';
        this.elements.serverList.style.display = 'flex';
        this.elements.serverModalFooter.style.display = 'flex';
    },
    
    async saveServer() {
        const server = {
            name: document.getElementById('server-name-input').value.trim(),
            url: document.getElementById('server-url-input').value.trim(),
            username: document.getElementById('server-username-input').value.trim(),
            password: document.getElementById('server-password-input').value
        };
        
        if (!server.url) {
            this.showError('URL is required');
            return;
        }
        
        try {
            const url = this.editingServerIndex >= 0 
                ? `/api/servers/${this.editingServerIndex}` 
                : '/api/servers';
            const method = this.editingServerIndex >= 0 ? 'PUT' : 'POST';
            
            const response = await fetch(url, {
                method,
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(server)
            });
            
            if (response.ok) {
                this.showToast('Server saved', 'success');
                this.fetchServers();
                this.cancelServerEdit();
            } else {
                const data = await response.json();
                this.showError(data.error || 'Failed to save server');
            }
        } catch (error) {
            this.showError('Failed to save server');
        }
    },
    
    async deleteServer(index) {
        if (!confirm('Are you sure you want to delete this server?')) return;
        
        try {
            const response = await fetch(`/api/servers/${index}`, { method: 'DELETE' });
            
            if (response.ok) {
                this.showToast('Server deleted', 'success');
                this.fetchServers();
            } else {
                this.showError('Failed to delete server');
            }
        } catch (error) {
            this.showError('Failed to delete server');
        }
    },
    
    async selectServer(index) {
        try {
            const response = await fetch(`/api/servers/${index}/activate`, { method: 'POST' });
            
            if (response.ok) {
                this.showToast('Server activated', 'success');
                this.fetchServers();
                this.fetchStatus();
                this.loadSection('resume');
            } else {
                this.showError('Failed to activate server');
            }
        } catch (error) {
            this.showError('Failed to activate server');
        }
    },
    
    async pingServers() {
        this.showToast('Testing server latency...', 'info');
        
        try {
            const response = await fetch('/api/servers/ping', { method: 'POST' });
            const data = await response.json();
            
            if (response.ok) {
                this.servers = data.servers || this.servers;
                this.renderServerList();
                this.showToast('Latency test complete', 'success');
            }
        } catch (error) {
            this.showError('Failed to test latency');
        }
    },
    
    // API Calls
    async fetchStatus() {
        try {
            const response = await fetch('/api/status');
            const data = await response.json();
            
            if (response.ok) {
                this.updateStatusBar(data);
            }
        } catch (error) {
            console.error('Status fetch error:', error);
        }
    },
    
    async fetchServers() {
        try {
            const response = await fetch('/api/servers');
            const data = await response.json();
            
            if (response.ok) {
                this.servers = data.servers || [];
                this.currentServer = data.active || null;
                this.renderServerList();
            }
        } catch (error) {
            console.error('Servers fetch error:', error);
        }
    },
    
    updateStatusBar(data) {
        // Server status
        const serverName = data.server?.name || data.server?.url || 'No server';
        this.elements.serverName.textContent = serverName;
        
        const statusDot = this.elements.serverStatus.querySelector('.status-dot');
        if (data.connected) {
            statusDot.classList.add('connected');
        } else {
            statusDot.classList.remove('connected');
        }
        
        // Latency
        this.elements.latency.textContent = data.latency ? `${data.latency}ms` : '--';
        
        // MPV status
        this.elements.mpvText.textContent = data.mpvAvailable ? 'Available' : 'Not found';
        this.elements.mpvText.style.color = data.mpvAvailable ? 'var(--success-color)' : 'var(--error-color)';
    },
    
    // UI Helpers
    showLoading() {
        this.elements.loadingState.style.display = 'flex';
        this.elements.mediaGrid.style.display = 'none';
        this.elements.emptyState.style.display = 'none';
        this.elements.pagination.style.display = 'none';
    },
    
    hideLoading() {
        this.elements.loadingState.style.display = 'none';
        this.elements.mediaGrid.style.display = 'grid';
    },
    
    showEmpty() {
        this.elements.loadingState.style.display = 'none';
        this.elements.mediaGrid.style.display = 'none';
        this.elements.emptyState.style.display = 'flex';
        this.elements.pagination.style.display = 'none';
    },
    
    hideEmpty() {
        this.elements.emptyState.style.display = 'none';
    },
    
    showBreadcrumb(title) {
        this.elements.breadcrumb.style.display = 'flex';
        this.elements.breadcrumbCurrent.textContent = title;
    },
    
    hideBreadcrumb() {
        this.elements.breadcrumb.style.display = 'none';
    },
    
    showToast(message, type = 'info') {
        const toast = document.createElement('div');
        toast.className = `toast ${type}`;
        toast.innerHTML = `
            <span class="toast-message">${message}</span>
            <button class="toast-close" onclick="this.parentElement.remove()">
                <svg width="14" height="14" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <line x1="18" y1="6" x2="6" y2="18"></line>
                    <line x1="6" y1="6" x2="18" y2="18"></line>
                </svg>
            </button>
        `;
        
        this.elements.toastContainer.appendChild(toast);
        
        setTimeout(() => {
            toast.style.opacity = '0';
            toast.style.transform = 'translateX(100%)';
            setTimeout(() => toast.remove(), 200);
        }, 5000);
    },
    
    showError(message) {
        this.showToast(message, 'error');
    },
    
    formatDuration(ticks) {
        if (!ticks) return '';
        const seconds = Math.floor(ticks / 10000000);
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        const secs = seconds % 60;
        
        if (hours > 0) {
            return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
        }
        return `${minutes}:${secs.toString().padStart(2, '0')}`;
    }
};

// Initialize app on DOM ready
document.addEventListener('DOMContentLoaded', () => app.init());
