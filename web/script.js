const app = {
    config: {
        api: {
            profiles: '/api/profiles',
            historyBase: '/api/history?owner='
        },
        messages: {
            noProfiles: 'STATUS: NO_PROFILES_FOUND.',
            noHistory: 'STATUS: NO_HISTORY_FOUND.',
            fetchError: (status) => `ERR_NET_FETCH (${status})`,
            fatalError: (msg) => `FATAL: ${msg}.`
        }
    },
    dom: {},
    state: {
        historyChart: null,
        modalRequestID: 0,
    },
    init() {
        this.dom.loader = document.getElementById('loader');
        this.dom.errorMessage = document.getElementById('error-message');
        this.dom.profilesContainer = document.getElementById('profiles');
        this.dom.historyModal = document.getElementById('historyModal');
        this.dom.modalOwnerName = document.getElementById('modal-owner-name');
        this.dom.modalCloseBtn = document.getElementById('modal-close-btn');
        this.dom.modalLoader = document.getElementById('modal-loader');
        this.dom.modalMessage = document.getElementById('modal-message');
        this.dom.chartContainer = document.getElementById('chartContainer');
        this.addEventListeners();
        this.fetchProfiles();
    },
    addEventListeners() {
        this.dom.profilesContainer.addEventListener('click', (e) => {
            const button = e.target.closest('.view-history-btn');
            if (button) this.showHistory(button.dataset.owner);
        });
        this.dom.modalCloseBtn.addEventListener('click', () => this.closeModal());
        this.dom.historyModal.addEventListener('click', (e) => {
            if (e.target === this.dom.historyModal) this.closeModal();
        });
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && this.dom.historyModal.classList.contains('visible')) {
                this.closeModal();
            }
        });
        this.dom.historyModal.addEventListener('keydown', (e) => this.trapFocus(e));
    },
    async fetchProfiles() {
        try {
            const response = await fetch(this.config.api.profiles);
            if (!response.ok) throw new Error(this.config.messages.fetchError(response.status));
            const profiles = await response.json();
            if (!profiles || profiles.length === 0) {
                this.showError(this.config.messages.noProfiles);
                return;
            }
            const fragment = document.createDocumentFragment();
            profiles.forEach(profile => {
                const article = document.createElement('article');
                article.className = "flex flex-col bg-opacity-20 bg-gray-700 border border-[#30363d] hover:border-gray-500 transition-colors duration-300";
                article.innerHTML = `

						<div class="p-5 flex-grow">
							<h3 class="text-xl font-bold mb-4 text-glow text-green-400">> ${profile.owner}</h3>
							<div class="grid grid-cols-2 gap-x-4 text-sm">
								<span class="text-gray-400"># Rank</span>
								<span class="font-semibold text-green-400 text-right">${profile.rank}</span>
								<span class="text-gray-400">^ Total Upload</span>
								<span class="font-semibold text-green-400 text-right">${profile.upload}</span>
								<span class="text-gray-400">+ Current Upload</span>
								<span class="font-semibold text-green-400 text-right">${profile.current_upload}</span>
								<span class="text-gray-400">- Current Download</span>
								<span class="font-semibold text-green-400 text-right">${profile.current_download}</span>
								<span class="text-gray-400">* Points</span>
								<span class="font-semibold text-green-400 text-right">${profile.points.toLocaleString()}</span>
								<span class="text-gray-400">~ Seeding</span>
								<span class="font-semibold text-green-400 text-right">${profile.seeding_count} torrents</span>
							</div>
						</div>
						<div class="p-5 pt-2">
							<button data-owner="${profile.owner}" class="view-history-btn action-btn w-full py-2 font-semibold">VIEW_HISTORY</button>
						</div>`;
                fragment.appendChild(article);
            });
            this.dom.profilesContainer.appendChild(fragment);
            this.dom.profilesContainer.style.display = 'grid';
            this.dom.loader.style.display = 'none';
        } catch (e) {
            this.showError(this.config.messages.fatalError(e.message));
            console.error(e);
        }
    },
    async showHistory(owner) {
        this.state.modalRequestID++;
        const currentRequestID = this.state.modalRequestID;
        this.dom.modalOwnerName.textContent = owner;
        this.dom.modalLoader.style.display = 'block';
        this.dom.chartContainer.style.display = 'none';
        this.dom.modalMessage.style.display = 'none';
        this.dom.historyModal.classList.add('visible');
        document.body.style.overflow = 'hidden';
        this.dom.modalCloseBtn.focus();
        try {
            const response = await fetch(`${this.config.api.historyBase}${encodeURIComponent(owner)}`);
            if (this.state.modalRequestID !== currentRequestID) return;
            if (!response.ok) throw new Error(this.config.messages.fetchError(response.status));
            const historyData = await response.json();
            if (this.state.modalRequestID !== currentRequestID) return;
            if (!historyData || historyData.length === 0) {
                this.showModalMessage(this.config.messages.noHistory);
            } else {
                this.dom.modalLoader.style.display = 'none';
                this.dom.chartContainer.style.display = 'block';
                this.renderChart(historyData);
            }
        } catch (e) {
            if (this.state.modalRequestID === currentRequestID) {
                this.showModalMessage(this.config.messages.fatalError(e.message));
                console.error(e);
            }
        }
    },
    renderChart(historyData) {
        if (this.state.historyChart) {
            this.state.historyChart.destroy();
        }
        this.dom.chartContainer.innerHTML = ' <canvas tabindex="0"> </canvas>';
        const canvas = this.dom.chartContainer.querySelector('canvas');
        if (!canvas) return;
        const parseUploadValue = (value) => {
            if (typeof value !== 'string') return 0;
            const num = parseFloat(value.replace(/,/g, '').replace(/TiB|GiB|MiB/i, '').trim());
            if (isNaN(num)) return 0;
            if (value.toLowerCase().includes('gib')) return num / 1024;
            if (value.toLowerCase().includes('mib')) return num / 1024 / 1024;
            return num;
        };
        const labels = historyData.map(r => new Date(r.timestamp).toLocaleDateString('en-CA'));
        const rankData = historyData.map(r => r.rank);
        const uploadData = historyData.map(r => parseUploadValue(r.upload));
        const pointsData = historyData.map(r => r.points);
        const seedingData = historyData.map(r => r.seeding_count);
        const textColor = getComputedStyle(document.documentElement).getPropertyValue('--text-primary').trim();
        const gridColor = 'rgba(50, 50, 50, 0.5)';
        const font = {
            family: "'Fira Code', monospace"
        };
        const accentGreen = getComputedStyle(document.documentElement).getPropertyValue('--accent-green').trim();
        const accentAmber = getComputedStyle(document.documentElement).getPropertyValue('--accent-amber').trim();
        const accentCyan = getComputedStyle(document.documentElement).getPropertyValue('--accent-cyan').trim();
        const accentMagenta = getComputedStyle(document.documentElement).getPropertyValue('--accent-magenta').trim();
        this.state.historyChart = new Chart(canvas, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Rank',
                    data: rankData,
                    borderColor: accentMagenta,
                    tension: 0.2,
                    yAxisID: 'yRank',
                    fill: false
                }, {
                    label: 'Upload (TiB)',
                    data: uploadData,
                    borderColor: accentGreen,
                    tension: 0.2,
                    yAxisID: 'yUpload',
                    fill: false
                }, {
                    label: 'Points',
                    data: pointsData,
                    borderColor: accentAmber,
                    tension: 0.2,
                    yAxisID: 'yPoints',
                    fill: false
                }, {
                    label: 'Seeding',
                    data: seedingData,
                    borderColor: accentCyan,
                    tension: 0.2,
                    yAxisID: 'ySeeding',
                    fill: false
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    mode: 'index',
                    intersect: false
                },
                scales: {
                    x: {
                        grid: {
                            color: gridColor,
                            borderDash: [2, 4]
                        },
                        ticks: {
                            color: textColor,
                            font: font
                        }
                    },
                    yRank: {
                        type: 'linear',
                        position: 'left',
                        reverse: true,
                        title: {
                            display: true,
                            text: 'Rank',
                            color: accentMagenta,
                            font: font
                        },
                        grid: {
                            drawOnChartArea: false
                        },
                        ticks: {
                            color: accentMagenta,
                            font: font
                        }
                    },
                    yUpload: {
                        type: 'linear',
                        position: 'left',
                        title: {
                            display: true,
                            text: 'Upload (TiB)',
                            color: accentGreen,
                            font: font
                        },
                        grid: {
                            color: gridColor,
                            borderDash: [2, 4]
                        },
                        ticks: {
                            color: accentGreen,
                            font: font
                        },
                        offset: true
                    },
                    yPoints: {
                        type: 'linear',
                        position: 'right',
                        title: {
                            display: true,
                            text: 'Points',
                            color: accentAmber,
                            font: font
                        },
                        grid: {
                            drawOnChartArea: false
                        },
                        ticks: {
                            color: accentAmber,
                            font: font
                        }
                    },
                    ySeeding: {
                        type: 'linear',
                        position: 'right',
                        title: {
                            display: true,
                            text: 'Seeding',
                            color: accentCyan,
                            font: font
                        },
                        grid: {
                            drawOnChartArea: false
                        },
                        ticks: {
                            color: accentCyan,
                            font: font
                        },
                        offset: true
                    }
                },
                plugins: {
                    legend: {
                        labels: {
                            color: textColor,
                            font: font
                        }
                    },
                    tooltip: {
                        backgroundColor: '#000',
                        titleFont: font,
                        bodyFont: font,
                        padding: 10,
                        cornerRadius: 0,
                        borderColor: textColor,
                        borderWidth: 1
                    }
                }
            }
        });
    },
    closeModal() {
        this.state.modalRequestID++;
        this.dom.historyModal.classList.remove('visible');
        document.body.style.overflow = 'auto';
        if (this.state.historyChart) {
            this.state.historyChart.destroy();
            this.state.historyChart = null;
        }
    },
    showError(message) {
        this.dom.loader.style.display = 'none';
        this.dom.errorMessage.textContent = message;
        this.dom.errorMessage.style.display = 'block';
    },
    showModalMessage(message) {
        this.dom.modalLoader.style.display = 'none';
        this.dom.chartContainer.style.display = 'none';
        this.dom.modalMessage.textContent = message;
        this.dom.modalMessage.style.display = 'block';
    },
    trapFocus(e) {
        if (e.key !== 'Tab') return;
        const focusableElements = this.dom.historyModal.querySelectorAll('button, [tabindex]:not([tabindex="-1"])');
        const firstElement = focusableElements[0];
        const lastElement = focusableElements[focusableElements.length - 1];
        if (e.shiftKey) {
            if (document.activeElement === firstElement) {
                lastElement.focus();
                e.preventDefault();
            }
        } else {
            if (document.activeElement === lastElement) {
                firstElement.focus();
                e.preventDefault();
            }
        }
    }
};
document.addEventListener('DOMContentLoaded', () => app.init());
