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
    const response = await fetch(`${config.api.history}${encodeURIComponent(owner)}`);
    if (!response.ok) throw new Error('Network error');
    const historyData = await response.json();

    if (!historyData || historyData.length === 0) {
      root.innerHTML = '<div class="spinner-container"><p class="stat-label">No history available.</p></div>';
      return;
    }

    const parseUploadValue = (value) => {
      if (typeof value !== 'string') return 0;
      const num = parseFloat(value.replace(/,/g, '').replace(/TiB|GiB|MiB/i, '').trim());
      if (isNaN(num)) return 0;
      const lowerVal = value.toLowerCase();
      if (lowerVal.includes('tib')) return num;
      if (lowerVal.includes('gib')) return num / 1024;
      if (lowerVal.includes('mib')) return num / 1024 / 1024;
      return num;
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
        foreColor: '#888',
        fontFamily: 'Inter, system-ui, sans-serif',
        toolbar: { show: false },
        animations: { enabled: true, easing: 'easeinout', speed: 800 }
      },
      theme: { mode: 'dark' },
      colors: ['#FFFFFF', '#00FF00', '#0066FF', '#FF00FF'],
      stroke: {
        curve: 'smooth',
        width: [3, 2, 2, 2],
        dashArray: 0
      },
      grid: {
        borderColor: '#1a1a1a',
        strokeDashArray: 0,
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
            style: { colors: '#FFF', fontSize: '10px', fontWeight: 700 },
            formatter: (v) => v != null ? v.toFixed(2) : ''
          }
        },
        {
          seriesName: 'Rank',
          opposite: true,
          reversed: true,
          labels: {
            style: { colors: '#00FF00', fontSize: '10px', fontWeight: 700 },
            formatter: (v) => v != null ? '#' + Math.round(v) : ''
          }
        },
        {
          seriesName: 'Points',
          show: false
        },
        {
          seriesName: 'Seeding',
          show: false
        }
      ],
      legend: {
        position: 'bottom',
        fontSize: '14px',
        fontWeight: 600,
        markers: { width: 10, height: 10, radius: 2 },
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
