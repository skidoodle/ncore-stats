const config = {
  api: {
    history: '/api/history?owner='
  }
};

let currentChart = null;

async function renderChart(owner) {
  const root = document.getElementById('modal-stats-root');
  if (!root) return;

  try {
    const response = await fetch(`${config.api.history}${encodeURIComponent(owner)}`); if (!response.ok) throw new Error('Network error');
    const historyData = await response.json();

    if (!historyData || historyData.length === 0) {
      root.innerHTML = '<p class="stat-label" style="text-align: center; padding: 2rem; color: var(--muted);">No history available.</p>';
      return;
    }
    const parseUploadValue = (value) => {
      if (typeof value !== 'string') return 0;
      const num = parseFloat(value.replace(/,/g, '').replace(/TiB|GiB|MiB|KiB|B/i, '').trim());
      if (isNaN(num)) return 0;
      const lowerVal = value.toLowerCase();
      if (lowerVal.includes('tib')) return num;
      if (lowerVal.includes('gib')) return num / 1024;
      if (lowerVal.includes('mib')) return num / 1024 / 1024;
      if (lowerVal.includes('kib')) return num / 1024 / 1024 / 1024;
      return num / 1024 / 1024 / 1024 / 1024;
    };

    const series = [
      {
        name: 'Upload',
        data: historyData.map(r => ({ x: new Date(r.timestamp).getTime(), y: parseUploadValue(r.upload) }))
      },
      {
        name: 'Rank',
        data: historyData.map(r => ({ x: new Date(r.timestamp).getTime(), y: r.rank }))
      },
      {
        name: 'Points',
        data: historyData.map(r => ({ x: new Date(r.timestamp).getTime(), y: r.points }))
      },
      {
        name: 'Seeding',
        data: historyData.map(r => ({ x: new Date(r.timestamp).getTime(), y: r.seeding_count }))
      }
    ];

    root.innerHTML = '<div id="chart-mount" style="height: 100%; width: 100%;"></div>';

    const options = {
      series: series,
      chart: {
        type: 'line',
        height: '100%',
        width: '100%',
        background: 'transparent',
        foreColor: '#71717a',
        fontFamily: 'Inter, system-ui, sans-serif',
        toolbar: { show: false },
        animations: { enabled: true, easing: 'easeinout', speed: 800 }
      },
      responsive: [
        {
          breakpoint: 700,
          options: {
            yaxis: [
              { show: false },
              { show: false },
              { show: false },
              { show: false }
            ],
            grid: {
              padding: { left: 0, right: 0 }
            }
          }
        }
      ],
      theme: { mode: 'dark' },
      colors: ['#FFFFFF', '#10b981', '#3b82f6', '#f43f5e'],
      stroke: {
        curve: 'smooth',
        width: [3, 2, 2, 2],
        dashArray: 0
      },
      grid: {
        borderColor: '#27272a',
        strokeDashArray: 4,
        padding: { top: 20, bottom: 0, left: 20, right: 20 },
        xaxis: { lines: { show: true } },
        yaxis: { lines: { show: true } }
      },
      xaxis: {
        type: 'datetime',
        labels: {
          style: { fontSize: '10px', fontWeight: 600 },
          datetimeUTC: false
        },
        axisBorder: { show: false },
        axisTicks: { show: false }
      },
      yaxis: [
        {
          seriesName: 'Upload',
          labels: {
            style: { colors: '#FFF', fontSize: '11px', fontWeight: 600 },
            formatter: (v) => v != null ? v.toFixed(2) : ''
          }
        },
        {
          seriesName: 'Points',
          opposite: false,
          labels: {
            style: { colors: '#3b82f6', fontSize: '11px', fontWeight: 600 },
            formatter: (v) => v != null ? Math.round(v).toLocaleString() : ''
          }
        },
        {
          seriesName: 'Rank',
          opposite: true,
          reversed: true,
          labels: {
            style: { colors: '#10b981', fontSize: '11px', fontWeight: 600 },
            formatter: (v) => v != null ? '#' + Math.round(v) : ''
          }
        },
        {
          seriesName: 'Seeding',
          opposite: true,
          labels: {
            style: { colors: '#f43f5e', fontSize: '11px', fontWeight: 600 },
            formatter: (v) => v != null ? Math.round(v) : ''
          }
        }
      ],
      legend: {
        position: 'bottom',
        fontSize: '12px',
        fontWeight: 600,
        markers: { width: 8, height: 8, radius: 10 },
        itemMargin: { horizontal: 15, vertical: 10 }
      },
      tooltip: {
        shared: true,
        theme: 'dark',
        x: { format: 'dd MMM yyyy HH:mm' },
        y: {
          formatter: (val, { seriesIndex }) => {
            if (val == null) return '--';
            switch (seriesIndex) {
              case 0: return val.toFixed(3) + ' TiB';
              case 1: return '#' + Math.round(val);
              default: return Math.round(val).toLocaleString();
            }
          }
        }
      },
      markers: { size: 0, hover: { size: 5 } }
    };

    if (currentChart) currentChart.destroy();
    currentChart = new ApexCharts(document.getElementById('chart-mount'), options);
    currentChart.render();

  } catch (e) {
    root.innerHTML = `<div class="spinner-container"><p class="stat-label" style="color: #ef4444;">Error: ${e.message}</p></div>`;
  }
}
