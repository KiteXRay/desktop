import React, { useMemo } from 'react';
import { formatSpeed } from '../utils/formatters';

interface NetworkChartProps {
  downloadHistory: number[]; // MB
  uploadHistory: number[];   // MB
  currentDownload: number;   // KB/s
  currentUpload: number;     // KB/s
  height?: number;
}

export const NetworkChart: React.FC<NetworkChartProps> = ({
  downloadHistory,
  uploadHistory,
  currentDownload,
  currentUpload,
  height = 130,
}) => {
  // Pad arrays to 60 points (1 point per second for last minute)
  const pointsCount = 60;

  const { uploadPoints, downloadPoints, maxY } = useMemo(() => {
    const ups = [...(uploadHistory || [])];
    const downs = [...(downloadHistory || [])];

    // Ensure we have at least pointsCount points
    while (ups.length < pointsCount) ups.unshift(0);
    while (downs.length < pointsCount) downs.unshift(0);

    const slicedUps = ups.slice(-pointsCount);
    const slicedDowns = downs.slice(-pointsCount);

    // Convert MB to KB/s for chart scaling
    const upKBs = slicedUps.map(v => v * 1024);
    const downKBs = slicedDowns.map(v => v * 1024);

    const maxVal = Math.max(
      ...upKBs,
      ...downKBs,
      currentUpload,
      currentDownload,
      10 // Minimum 10 KB/s scale
    );

    return {
      uploadPoints: upKBs,
      downloadPoints: downKBs,
      maxY: maxVal * 1.15, // 15% padding at top
    };
  }, [uploadHistory, downloadHistory, currentUpload, currentDownload]);

  const width = 450;
  const paddingBottom = 15;
  const chartHeight = height - paddingBottom;

  const buildPath = (data: number[]) => {
    if (data.length === 0) return '';
    const step = width / (data.length - 1);

    const coords = data.map((val, idx) => {
      const x = idx * step;
      const y = chartHeight - (val / maxY) * chartHeight;
      return [x, Math.max(2, Math.min(chartHeight, y))];
    });

    // Generate smooth cubic bezier curve
    let d = `M ${coords[0][0]} ${coords[0][1]}`;
    for (let i = 0; i < coords.length - 1; i++) {
      const x_mid = (coords[i][0] + coords[i + 1][0]) / 2;
      const y_mid = (coords[i][1] + coords[i + 1][1]) / 2;
      const cp_x1 = (x_mid + coords[i][0]) / 2;
      const cp_x2 = (x_mid + coords[i + 1][0]) / 2;
      d += ` Q ${cp_x1} ${coords[i][1]}, ${x_mid} ${y_mid}`;
      d += ` Q ${cp_x2} ${coords[i + 1][1]}, ${coords[i + 1][0]} ${coords[i + 1][1]}`;
    }
    return d;
  };

  const uploadLine = buildPath(uploadPoints);
  const downloadLine = buildPath(downloadPoints);

  const uploadArea = `${uploadLine} L ${width} ${chartHeight} L 0 ${chartHeight} Z`;
  const downloadArea = `${downloadLine} L ${width} ${chartHeight} L 0 ${chartHeight} Z`;

  return (
    <div className="w-full bg-slate-900/60 rounded-xl p-3.5 border border-slate-800/80 shadow-inner">
      <div className="flex items-center justify-between mb-2 px-1">
        <div className="flex items-center gap-4 text-xs">
          <div className="flex items-center gap-1.5 font-medium">
            <span className="w-2.5 h-2.5 rounded-full bg-sky-500 shadow-sm shadow-sky-500/50 animate-pulse" />
            <span className="text-slate-400">Download:</span>
            <span className="text-sky-400 font-mono">{formatSpeed(currentDownload)}</span>
          </div>
          <div className="flex items-center gap-1.5 font-medium">
            <span className="w-2.5 h-2.5 rounded-full bg-emerald-500 shadow-sm shadow-emerald-500/50 animate-pulse" />
            <span className="text-slate-400">Upload:</span>
            <span className="text-emerald-400 font-mono">{formatSpeed(currentUpload)}</span>
          </div>
        </div>
        <div className="text-[11px] font-mono text-slate-500">
          Peak: {formatSpeed(maxY)}
        </div>
      </div>

      <div className="relative w-full overflow-hidden" style={{ height }}>
        <svg
          viewBox={`0 0 ${width} ${height}`}
          className="w-full h-full overflow-visible"
          preserveAspectRatio="none"
        >
          <defs>
            <linearGradient id="grad-upload" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#10b981" stopOpacity="0.3" />
              <stop offset="100%" stopColor="#10b981" stopOpacity="0.0" />
            </linearGradient>
            <linearGradient id="grad-download" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#0ea5e9" stopOpacity="0.35" />
              <stop offset="100%" stopColor="#0ea5e9" stopOpacity="0.0" />
            </linearGradient>
          </defs>

          {/* Grid lines */}
          <line x1="0" y1={chartHeight * 0.25} x2={width} y2={chartHeight * 0.25} stroke="#334155" strokeWidth="0.5" strokeDasharray="3 3" opacity="0.4" />
          <line x1="0" y1={chartHeight * 0.5} x2={width} y2={chartHeight * 0.5} stroke="#334155" strokeWidth="0.5" strokeDasharray="3 3" opacity="0.4" />
          <line x1="0" y1={chartHeight * 0.75} x2={width} y2={chartHeight * 0.75} stroke="#334155" strokeWidth="0.5" strokeDasharray="3 3" opacity="0.4" />
          <line x1="0" y1={chartHeight} x2={width} y2={chartHeight} stroke="#475569" strokeWidth="1" opacity="0.5" />

          {/* Download fill and line */}
          <path d={downloadArea} fill="url(#grad-download)" />
          <path d={downloadLine} fill="none" stroke="#38bdf8" strokeWidth="1.8" strokeLinecap="round" />

          {/* Upload fill and line */}
          <path d={uploadArea} fill="url(#grad-upload)" />
          <path d={uploadLine} fill="none" stroke="#34d399" strokeWidth="1.8" strokeLinecap="round" />
        </svg>
      </div>
    </div>
  );
};
