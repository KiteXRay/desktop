import React, { useRef, useEffect, useMemo } from 'react';
import { formatSpeed } from '../utils/formatters';

interface NetworkChartProps {
  downloadHistory: number[]; // MB
  uploadHistory: number[];   // MB
  currentDownload: number;   // KB/s
  currentUpload: number;     // KB/s
  height?: number;
}

const pointsCount = 60;

export const NetworkChart: React.FC<NetworkChartProps> = React.memo(({
  downloadHistory,
  uploadHistory,
  currentDownload,
  currentUpload,
  height = 130,
}) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);

  const { uploadPoints, downloadPoints, maxY } = useMemo(() => {
    const ups = [...(uploadHistory || [])];
    const downs = [...(downloadHistory || [])];

    while (ups.length < pointsCount) ups.unshift(0);
    while (downs.length < pointsCount) downs.unshift(0);

    const slicedUps = ups.slice(-pointsCount);
    const slicedDowns = downs.slice(-pointsCount);

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
      maxY: maxVal * 1.15,
    };
  }, [uploadHistory, downloadHistory, currentUpload, currentDownload]);

  const paddingBottom = 15;
  const chartHeight = height - paddingBottom;

  useEffect(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;

    // Skip drawing if window is minimized or hidden in background to save CPU
    if (document.hidden) return;

    const rect = container.getBoundingClientRect();
    const width = Math.max(rect.width, 200);
    const dpr = Math.min(window.devicePixelRatio || 1, 2);

    if (canvas.width !== Math.floor(width * dpr) || canvas.height !== Math.floor(height * dpr)) {
      canvas.width = Math.floor(width * dpr);
      canvas.height = Math.floor(height * dpr);
      canvas.style.width = `${width}px`;
      canvas.style.height = `${height}px`;
    }

    const ctx = canvas.getContext('2d', { alpha: true });
    if (!ctx) return;

    ctx.save();
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, width, height);

    // Draw dashed grid lines
    ctx.setLineDash([3, 3]);
    ctx.strokeStyle = '#334155';
    ctx.lineWidth = 0.5;
    ctx.globalAlpha = 0.4;

    const yLevels = [chartHeight * 0.25, chartHeight * 0.5, chartHeight * 0.75];
    yLevels.forEach(y => {
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(width, y);
      ctx.stroke();
    });

    // Solid baseline
    ctx.setLineDash([]);
    ctx.strokeStyle = '#475569';
    ctx.lineWidth = 1;
    ctx.globalAlpha = 0.5;
    ctx.beginPath();
    ctx.moveTo(0, chartHeight);
    ctx.lineTo(width, chartHeight);
    ctx.stroke();

    ctx.globalAlpha = 1;

    const drawCurve = (data: number[]) => {
      if (data.length < 2) return;
      const step = width / (data.length - 1);
      const coords = data.map((val, idx) => [
        idx * step,
        Math.max(2, Math.min(chartHeight, chartHeight - (val / maxY) * chartHeight)),
      ]);

      ctx.moveTo(coords[0][0], coords[0][1]);
      for (let i = 0; i < coords.length - 1; i++) {
        const x_mid = (coords[i][0] + coords[i + 1][0]) / 2;
        const y_mid = (coords[i][1] + coords[i + 1][1]) / 2;
        ctx.quadraticCurveTo(coords[i][0], coords[i][1], x_mid, y_mid);
      }
      ctx.lineTo(coords[coords.length - 1][0], coords[coords.length - 1][1]);
    };

    // 1. Download Area & Stroke (Sky Blue)
    const downGrad = ctx.createLinearGradient(0, 0, 0, chartHeight);
    downGrad.addColorStop(0, 'rgba(14, 165, 233, 0.35)');
    downGrad.addColorStop(1, 'rgba(14, 165, 233, 0.0)');

    ctx.beginPath();
    drawCurve(downloadPoints);
    ctx.lineTo(width, chartHeight);
    ctx.lineTo(0, chartHeight);
    ctx.closePath();
    ctx.fillStyle = downGrad;
    ctx.fill();

    ctx.beginPath();
    drawCurve(downloadPoints);
    ctx.strokeStyle = '#38bdf8';
    ctx.lineWidth = 1.8;
    ctx.lineCap = 'round';
    ctx.stroke();

    // 2. Upload Area & Stroke (Emerald)
    const upGrad = ctx.createLinearGradient(0, 0, 0, chartHeight);
    upGrad.addColorStop(0, 'rgba(16, 185, 129, 0.30)');
    upGrad.addColorStop(1, 'rgba(16, 185, 129, 0.0)');

    ctx.beginPath();
    drawCurve(uploadPoints);
    ctx.lineTo(width, chartHeight);
    ctx.lineTo(0, chartHeight);
    ctx.closePath();
    ctx.fillStyle = upGrad;
    ctx.fill();

    ctx.beginPath();
    drawCurve(uploadPoints);
    ctx.strokeStyle = '#34d399';
    ctx.lineWidth = 1.8;
    ctx.lineCap = 'round';
    ctx.stroke();

    ctx.restore();
  }, [uploadPoints, downloadPoints, maxY, height, chartHeight]);

  return (
    <div className="w-full bg-slate-900/60 rounded-xl p-3.5 border border-slate-800/80 shadow-inner">
      <div className="flex items-center justify-between mb-2 px-1">
        <div className="flex items-center gap-4 text-xs">
          <div className="flex items-center gap-1.5 font-medium">
            <span className="w-2 h-2 rounded-full bg-sky-400 shadow-[0_0_6px_#38bdf8]" />
            <span className="text-slate-400">Download:</span>
            <span className="text-sky-400 font-mono">{formatSpeed(currentDownload)}</span>
          </div>
          <div className="flex items-center gap-1.5 font-medium">
            <span className="w-2 h-2 rounded-full bg-emerald-400 shadow-[0_0_6px_#34d399]" />
            <span className="text-slate-400">Upload:</span>
            <span className="text-emerald-400 font-mono">{formatSpeed(currentUpload)}</span>
          </div>
        </div>
        <div className="text-[11px] font-mono text-slate-500">
          Peak: {formatSpeed(maxY)}
        </div>
      </div>

      <div ref={containerRef} className="relative w-full overflow-hidden" style={{ height }}>
        <canvas ref={canvasRef} className="w-full h-full block" />
      </div>
    </div>
  );
});
