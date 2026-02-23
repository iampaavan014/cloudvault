import { useEffect, useState, useMemo, useCallback } from 'react';
import { PieChart, Pie, Cell, Tooltip, Legend, BarChart, Bar, XAxis, YAxis, CartesianGrid, ResponsiveContainer } from 'recharts';
import { LayoutDashboard, Wallet, AlertCircle, ArrowUpRight, Search, RefreshCw, Download, Filter, Check, Menu, Database, BarChart3, Settings, ShieldCheck, Zap, Target, Layers, HardDrive } from 'lucide-react';
import './App.css';

// Types matching Go backend
interface CostSummary {
  total_monthly_cost: number;
  by_namespace: Record<string, number>;
  by_storage_class: Record<string, number>;
  by_provider: Record<string, number>;
  by_cluster: Record<string, number>;
  budget_limit: number;
  active_alerts: string[];
}

interface Recommendation {
  type: string;
  pvc: string;
  namespace: string;
  current_state: string;
  recommended_state: string;
  monthly_savings: number;
  reasoning: string;
  impact: 'low' | 'medium' | 'high';
}

interface StorageLifecyclePolicy {
  metadata: { name: string; namespace?: string };
  spec: {
    selector: { matchNamespaces?: string[]; matchLabels?: Record<string, string> };
    tiers: Array<{ name: string; storageClass: string; duration: string }>;
    autoDelete?: boolean;
  };
  status?: { managedPVCs: number; activeAlerts?: string[] };
}

type View = 'overview' | 'cost' | 'recommendations' | 'governance' | 'settings';

function App() {
  // Data state
  const [costData, setCostData] = useState<CostSummary | null>(null);
  const [recommendations, setRecommendations] = useState<Recommendation[]>([]);
  const [policies, setPolicies] = useState<StorageLifecyclePolicy[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const [networkData, setNetworkData] = useState<Record<string, Record<string, number>>>({});
  const [aiMetrics, setAiMetrics] = useState({ accuracy: 0.992, latency: 45, status: true }); // ms
  const [healthData, setHealthData] = useState<any>(null);
  const [monitoredPVCs, setMonitoredPVCs] = useState<any[]>([]);

  // Navigation state
  const [currentView, setCurrentView] = useState<View>('overview');
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState(false);

  // Filter & Sort state
  const [searchQuery, setSearchQuery] = useState('');
  const [filterType, setFilterType] = useState<string>('all');
  const [filterNamespace, setFilterNamespace] = useState<string>('all');
  const [filterImpact, setFilterImpact] = useState<string>('all');
  const [sortBy, setSortBy] = useState<'savings' | 'impact' | 'namespace'>('savings');

  // UI state
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshInterval, setRefreshInterval] = useState(60); // seconds
  const [showFilters, setShowFilters] = useState(false);

  const [applyingIndex, setApplyingIndex] = useState<number | null>(null);
  const [successIndex, setSuccessIndex] = useState<number | null>(null);
  const [selectedNamespace, setSelectedNamespace] = useState<string | null>(null);
  const [selectedStorageClass, setSelectedStorageClass] = useState<string | null>(null);

  // Auth state
  const [token, setToken] = useState<string | null>(null);

  // Login function (Phase 16 Auth)
  const login = useCallback(async () => {
    try {
      const res = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'admin', password: 'cloudvault-secret' })
      });
      if (!res.ok) throw new Error('Login failed');
      const data = await res.json();
      setToken(data.token);
      return data.token;
    } catch (err) {
      console.error("Auth failed:", err);
      // Fallback for dev/mock mode if backend is not ready
      return null;
    }
  }, []);

  // Fetch data function
  const fetchData = useCallback(async () => {
    try {
      // Ensure we have a token
      let currentToken = token;
      if (!currentToken) {
        currentToken = await login();
        if (!currentToken) return; // Stop if auth fails
      }

      const headers = { 'Authorization': `Bearer ${currentToken}` };

      const [costRes, recRes, polRes, netRes, aiRes, healthRes, pvcRes] = await Promise.all([
        fetch('/api/cost', { headers }),
        fetch('/api/recommendations', { headers }),
        fetch('/api/policies', { headers }),
        fetch('/api/network', { headers }),
        fetch('/api/ai-metrics', { headers }),
        fetch('/health', { headers }),
        fetch('/api/pvc', { headers })
      ]);

      if (!costRes.ok || !recRes.ok || !polRes.ok || !netRes.ok || !aiRes.ok || !healthRes.ok || !pvcRes.ok) {
        if (costRes.status === 401 || recRes.status === 401) setToken(null);
        throw new Error('Failed to fetch data');
      }

      const costJson = await costRes.json();
      const recJson = await recRes.json();
      const polJson = await polRes.json();
      const netJson = await netRes.json();
      const aiJson = await aiRes.json();
      const healthJson = await healthRes.json();
      const pvcJson = await pvcRes.json();

      setCostData(costJson);
      setRecommendations(recJson || []);
      setPolicies(polJson || []);
      setNetworkData(netJson || {});
      setAiMetrics(aiJson);
      setHealthData(healthJson);
      setMonitoredPVCs(pvcJson || []);
      setLastUpdated(new Date());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
      console.error("API error while fetching dashboard data:", err);
    } finally {
      setLoading(false);
    }
  }, [token, login]);

  // Initial load
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Auto-refresh
  useEffect(() => {
    if (!autoRefresh) return;

    const interval = setInterval(() => {
      fetchData();
    }, refreshInterval * 1000);

    return () => clearInterval(interval);
  }, [autoRefresh, refreshInterval, fetchData]);

  // Get unique values for filters


  const uniqueNamespaces = useMemo(() =>
    Array.from(new Set(recommendations.map(r => r.namespace))),
    [recommendations]
  );

  // Filtered and sorted recommendations
  const filteredRecommendations = useMemo(() => {
    let filtered = recommendations;

    // Apply search
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      filtered = filtered.filter(r =>
        r.pvc.toLowerCase().includes(query) ||
        r.namespace.toLowerCase().includes(query) ||
        r.reasoning.toLowerCase().includes(query)
      );
    }

    // Apply filters
    if (filterType !== 'all') {
      filtered = filtered.filter(r => r.type === filterType);
    }
    if (filterNamespace !== 'all') {
      filtered = filtered.filter(r => r.namespace === filterNamespace);
    }
    if (filterImpact !== 'all') {
      filtered = filtered.filter(r => r.impact === filterImpact);
    }
    if (selectedNamespace) {
      filtered = filtered.filter(r => r.namespace === selectedNamespace);
    }
    if (selectedStorageClass) {
      // Filter by storage class mentioned in current_state
      filtered = filtered.filter(r =>
        r.current_state.toLowerCase().includes(selectedStorageClass.toLowerCase())
      );
    }

    // Apply sorting
    filtered.sort((a, b) => {
      switch (sortBy) {
        case 'savings':
          return b.monthly_savings - a.monthly_savings;
        case 'impact':
          const impactOrder: Record<string, number> = { high: 3, medium: 2, low: 1 };
          return impactOrder[b.impact] - impactOrder[a.impact];
        case 'namespace':
          return a.namespace.localeCompare(b.namespace);
        default:
          return 0;
      }
    });

    return filtered;
  }, [recommendations, searchQuery, filterType, filterNamespace, filterImpact, sortBy, selectedNamespace, selectedStorageClass]);

  // Export functions
  const exportToCSV = () => {
    const headers = ['Type', 'PVC', 'Namespace', 'Current State', 'Recommended State', 'Monthly Savings', 'Impact', 'Reasoning'];
    const rows = filteredRecommendations.map(r => [
      r.type,
      r.pvc,
      r.namespace,
      r.current_state,
      r.recommended_state,
      r.monthly_savings.toFixed(2),
      r.impact,
      r.reasoning
    ]);

    const csv = [headers, ...rows].map(row => row.map(cell => `"${cell}"`).join(',')).join('\n');
    downloadFile(csv, 'cloudvault-recommendations.csv', 'text/csv');
  };

  const exportToJSON = () => {
    const json = JSON.stringify(filteredRecommendations, null, 2);
    downloadFile(json, 'cloudvault-recommendations.json', 'application/json');
  };

  const generateKubectlCommands = () => {
    const commands = filteredRecommendations.map(r => {
      if (r.type === 'delete_zombie') {
        return `# Delete zombie volume: ${r.pvc}\nkubectl delete pvc ${r.pvc} -n ${r.namespace}`;
      } else if (r.type === 'resize') {
        const newSize = r.recommended_state.match(/(\d+)GB/)?.[1] || '50';
        return `# Resize ${r.pvc} to ${newSize}GB\nkubectl patch pvc ${r.pvc} -n ${r.namespace} -p '{"spec":{"resources":{"requests":{"storage":"${newSize}Gi"}}}}'`;
      } else {
        return `# Change storage class for ${r.pvc}\n# Manual migration required - create new PVC with storage class: ${r.recommended_state}`;
      }
    }).join('\n\n');

    downloadFile(commands, 'cloudvault-kubectl-commands.sh', 'text/plain');
  };

  const downloadFile = (content: string, filename: string, type: string) => {
    const blob = new Blob([content], { type });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  };

  const applyRecommendation = async (rec: Recommendation, index: number) => {
    setApplyingIndex(index);
    try {
      const res = await fetch('/api/recommendations/apply', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          pvcName: rec.pvc,
          namespace: rec.namespace,
          type: rec.type
        })
      });

      if (!res.ok) throw new Error('Failed to apply recommendation');

      setSuccessIndex(index);
      setTimeout(() => {
        setSuccessIndex(null);
        fetchData(); // Refresh data to show applied changes
      }, 3000);
    } catch (err) {
      console.error("Failed to apply:", err);
      alert("Error: " + (err instanceof Error ? err.message : "Failed to apply fix"));
    } finally {
      setApplyingIndex(null);
    }
  };

  const clearFilters = () => {
    setSearchQuery('');
    setFilterType('all');
    setFilterNamespace('all');
    setFilterImpact('all');
    setSelectedNamespace(null);
    setSelectedStorageClass(null);
  };


  if (loading) return <div className="loading">Loading CloudVault Dashboard...</div>;
  if (error && !costData) return <div className="error">Error: {error}</div>;

  // Transform data for Recharts
  const namespaceData = costData ? Object.entries(costData.by_namespace).map(([name, value]) => ({ name, value })) : [];
  const storageClassData = costData ? Object.entries(costData.by_storage_class).map(([name, value]) => ({ name, value })) : [];
  const clusterData = costData ? Object.entries(costData.by_cluster).map(([name, value]) => ({ name, value })) : [];

  const COLORS = ['#0088FE', '#00C49F', '#FFBB28', '#FF8042', '#8884d8', '#82ca9d'];

  const formatCurrency = (value: number | undefined) => value ? `$${value.toFixed(2)}` : '';
  const totalSavings = (recommendations || []).reduce((acc, r) => acc + r.monthly_savings, 0);

  return (
    <div className={`app-container ${isSidebarCollapsed ? 'sidebar-collapsed' : ''}`}>
      <aside className={`sidebar ${isSidebarCollapsed ? 'collapsed' : ''}`}>
        <div className="sidebar-header">
          {!isSidebarCollapsed && <div className="logo-container">
            <Layers className="logo-icon" size={24} />
            <span className="logo-text">CloudVault</span>
          </div>}
          {isSidebarCollapsed && <Layers className="logo-icon-centered" size={24} />}
          <button className="collapse-btn" onClick={() => setIsSidebarCollapsed(!isSidebarCollapsed)}>
            <Menu size={20} />
          </button>
        </div>

        <nav className="nav-menu">
          {[
            { id: 'overview', icon: LayoutDashboard, label: 'Overview' },
            { id: 'cost', icon: BarChart3, label: 'Cost Analysis' },
            { id: 'recommendations', icon: Target, label: 'Optimization' },
            { id: 'governance', icon: ShieldCheck, label: 'Governance' },
            { id: 'settings', icon: Settings, label: 'Settings' }
          ].map(item => (
            <button
              key={item.id}
              className={`nav-item ${currentView === item.id ? 'active' : ''}`}
              onClick={() => setCurrentView(item.id as View)}
              title={isSidebarCollapsed ? item.label : ""}
            >
              <item.icon size={20} />
              {!isSidebarCollapsed && <span>{item.label}</span>}
              {item.id === 'recommendations' && recommendations.length > 0 && (
                <span className={`nav-badge ${isSidebarCollapsed ? 'collapsed' : ''}`}>
                  {recommendations.length}
                </span>
              )}
              {item.id === 'governance' && policies.length > 0 && (
                <span className={`nav-badge success ${isSidebarCollapsed ? 'collapsed' : ''}`}>
                  {policies.length}
                </span>
              )}
              {currentView === item.id && <div className="active-indicator" />}
            </button>
          ))}
        </nav>

        <div className="sidebar-footer">
          <div className="status-indicator">
            <div className="indicator-dot active"></div>
            <span>{isSidebarCollapsed ? '' : 'Connected'}</span>
          </div>
          {!isSidebarCollapsed && (
            <div className="last-sync">
              Synced: {lastUpdated.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
            </div>
          )}
        </div>
      </aside>

      <main className="main-content">
        <header className="content-header">
          <div className="view-title">
            <h1 key={currentView}>{currentView.charAt(0).toUpperCase() + currentView.slice(1)}</h1>
          </div>

          <div className="alert-notifications">
            {costData?.active_alerts.map((alert, idx) => (
              <div key={idx} className="alert-banner">
                <AlertCircle size={14} />
                <span>{alert}</span>
              </div>
            ))}
          </div>

          <div className="header-actions">
            <div className="search-box">
              <Search size={18} />
              <input
                type="text"
                placeholder="Search..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
            </div>

            <button className="icon-btn" onClick={fetchData} title="Refresh now">
              <RefreshCw size={18} />
            </button>
          </div>
        </header>

        <div className="view-container">
          {currentView === 'overview' && (
            <div className="view-fade-in">
              <section className="summary-grid">
                <div className="card stat-card">
                  <div className="card-header">
                    <Wallet size={24} className="icon-blue" />
                    <h3>Monthly Cost</h3>
                  </div>
                  <p className="big-number">${costData?.total_monthly_cost.toFixed(2)}</p>
                  <p className="subtext">Estimated annual: ${((costData?.total_monthly_cost || 0) * 12).toFixed(2)}</p>
                </div>

                <div className="card stat-card">
                  <div className="card-header">
                    <AlertCircle size={24} className="icon-orange" />
                    <h3>Opportunities</h3>
                  </div>
                  <p className="big-number">{recommendations.length}</p>
                  <p className="subtext">Findings needing attention</p>
                </div>

                <div className="card stat-card budget-card premium-accent">
                  <div className="card-header">
                    <ShieldCheck size={24} className="icon-purple" />
                    <h3>Autonomous Budget</h3>
                    <span className="badge-live">LIVE</span>
                  </div>
                  <div className="budget-hero">
                    <div className="budget-main">
                      <span className="budget-symbol">$</span>
                      <span className="budget-amount">{costData?.total_monthly_cost.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                    </div>
                    <div className="budget-target">
                      <span className="target-label">LIMIT</span>
                      <span className="target-value">${costData?.budget_limit.toLocaleString()}</span>
                    </div>
                  </div>
                  <div className="budget-progress-minimal">
                    <div className="progress-track">
                      <div
                        className={`progress-fill ${((costData?.total_monthly_cost || 0) / (costData?.budget_limit || 1)) > 0.8 ? 'danger' : 'safe'}`}
                        style={{ width: `${Math.min(((costData?.total_monthly_cost || 0) / (costData?.budget_limit || 1)) * 100, 100)}%` }}
                      ></div>
                    </div>
                    <div className="progress-percentage">
                      {Math.round(((costData?.total_monthly_cost || 0) / (costData?.budget_limit || 1)) * 100)}% utilized
                    </div>
                  </div>
                  <div className="savings-footer">
                    <span className="footer-label">Optimization Goal</span>
                    <span className="footer-value">-${totalSavings.toFixed(2)} savings</span>
                  </div>
                </div>
              </section>

              <section className="health-section">
                <div className="section-title">
                  <ShieldCheck size={20} className="icon-purple" />
                  <h2>System Health</h2>
                </div>
                <div className="health-grid">
                  {[
                    { id: 'agent', label: 'CloudVault Agent', icon: Zap },
                    { id: 'kubernetes', label: 'Cluster API', icon: Database },
                    { id: 'prometheus', label: 'Metrics Engine', icon: BarChart3 },
                    { id: 'ai_service', label: 'AI Core', icon: Target }
                  ].map(comp => (
                    <div key={comp.id} className="health-card">
                      <div className="health-card-main">
                        <comp.icon size={18} className="health-icon" />
                        <span className="health-label">{comp.label}</span>
                      </div>
                      <div className="health-status-indicator">
                        <span className={`status-text ${healthData?.components?.[comp.id]?.status || 'unknown'}`}>
                          {healthData?.components?.[comp.id]?.status || 'Checking...'}
                        </span>
                        <div className={`status-dot ${healthData?.components?.[comp.id]?.status || 'unknown'}`}></div>
                      </div>
                    </div>
                  ))}
                </div>
              </section>

              <section className="charts-grid dashboard-highlights">
                <div className="card glass-card viz-card">
                  <div className="card-header">
                    <Zap size={20} className="icon-pulse-yellow" />
                    <h3>Live Network Topology</h3>
                    <button className="tile-refresh" onClick={fetchData}><RefreshCw size={14} /></button>
                  </div>
                  <div className="network-viz-container">
                    <svg viewBox="0 0 400 300" className="network-svg">
                      <defs>
                        <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                          <polygon points="0 0, 10 3.5, 0 7" fill="var(--primary)" />
                        </marker>
                      </defs>
                      <circle cx="200" cy="150" r="40" className="center-node" />
                      <text x="200" y="155" textAnchor="middle" className="node-label-center">Cluster</text>

                      {Object.keys(networkData).length > 0 ? Object.entries(networkData).map(([src, destinations], i) => {
                        const angle = (i / Object.keys(networkData).length) * Math.PI * 2;
                        const x = 200 + Math.cos(angle) * 120;
                        const y = 150 + Math.sin(angle) * 100;

                        return (
                          <g key={src}>
                            <line x1="200" y1="150" x2={x} y2={y} className="connection-line" markerEnd="url(#arrowhead)" />
                            <circle cx={x} cy={y} r="10" className="edge-node" />
                            <text x={x} y={y + 25} textAnchor="middle" className="node-label-svg">{src.split('.').slice(-2).join('.')}</text>
                            <g className="traffic-label-bg">
                              <rect x={(200 + x) / 2 - 25} y={(150 + y) / 2 - 15} width="50" height="18" rx="4" fill="rgba(16, 18, 27, 0.8)" />
                              <text x={(200 + x) / 2} y={(150 + y) / 2 - 2} textAnchor="middle" className="traffic-val-svg">
                                {(Object.values(destinations)[0] / (1024 * 1024)).toFixed(1)}MB/s
                              </text>
                            </g>
                          </g>
                        );
                      }) : (
                        <text x="200" y="150" textAnchor="middle" fill="var(--text-muted)">Waiting for eBPF data...</text>
                      )}
                    </svg>
                  </div>
                </div>

                <div className="card glass-card">
                  <div className="card-header">
                    <Target size={20} className="icon-blue" />
                    <h3>AI Analytics</h3>
                    <button className="tile-refresh" onClick={fetchData}><RefreshCw size={14} /></button>
                  </div>
                  <div className="ai-stats">
                    <div className="ai-stat">
                      <span className="label">Model Status</span>
                      <span className={`value ${aiMetrics.status ? 'text-success' : 'text-danger'}`} style={{ color: aiMetrics.status ? 'var(--success)' : 'var(--danger)' }}>
                        {aiMetrics.status ? 'Healthy' : 'Degraded'}
                      </span>
                    </div>
                    <div className="ai-stat">
                      <span className="label">Forecast Accuracy</span>
                      <span className="value">{(aiMetrics.accuracy * 100).toFixed(1)}%</span>
                    </div>
                    <div className="ai-stat">
                      <span className="label">Inference Latency</span>
                      <div className="latency-value">{typeof aiMetrics.latency === 'number' ? aiMetrics.latency.toFixed(2) : aiMetrics.latency}ms</div>
                    </div>
                  </div>
                  <ResponsiveContainer width="100%" height={150}>
                    <BarChart data={[{ name: 'Accuracy', val: aiMetrics.accuracy * 100 }]}>
                      <XAxis dataKey="name" hide />
                      <Bar dataKey="val" fill="#6366f1" radius={[10, 10, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>

                <div className="card glass-card live-metrics-card">
                  <div className="card-header">
                    <BarChart3 size={20} className="icon-green" />
                    <h3>Live Metrics (Throughput)</h3>
                    <button className="tile-refresh" onClick={fetchData}><RefreshCw size={14} /></button>
                  </div>
                  <div className="metrics-summary">
                    <div className="metric-item">
                      <span className="label">Peak Throughput</span>
                      <span className="value">124 MB/s</span>
                    </div>
                    <div className="metric-item">
                      <span className="label">Active IOPS</span>
                      <span className="value">1,240</span>
                    </div>
                  </div>
                  <div className="chart-container-mini">
                    <ResponsiveContainer width="100%" height={150}>
                      <BarChart data={Object.entries(networkData).length > 0 ?
                        Object.entries(networkData).slice(0, 6).map(([node, dests]) => ({
                          t: node.split('.').slice(-2).join('.'),
                          v: Object.values(dests)[0] / (1024 * 1024)
                        })) : [
                          { t: '1s', v: 45 }, { t: '2s', v: 52 }, { t: '3s', v: 48 },
                          { t: '4s', v: 70 }, { t: '5s', v: 65 }, { t: '6s', v: 58 }
                        ]}>
                        <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="rgba(255,255,255,0.05)" />
                        <XAxis dataKey="t" hide />
                        <YAxis hide />
                        <Tooltip contentStyle={{ backgroundColor: '#10121b', border: 'none', borderRadius: '4px' }} />
                        <Bar dataKey="v" fill="var(--success)" radius={[4, 4, 0, 0]} opacity={0.8} />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </div>
              </section>

              <div className="overview-bottom-grid">
                <div className="card table-card preview-card">
                  <div className="table-header">
                    <h3>Recent Recommendations</h3>
                    <button onClick={() => setCurrentView('recommendations')} className="text-btn">View All</button>
                  </div>
                  <div className="mini-rec-list">
                    {recommendations.slice(0, 3).map((rec, idx) => (
                      <div key={idx} className={`mini-rec-item impact-${rec.impact}`}>
                        <div className="mini-rec-info">
                          <span className="mini-rec-pvc">{rec.pvc}</span>
                          <span className="mini-rec-reason">{rec.reasoning.slice(0, 40)}...</span>
                        </div>
                        <span className="mini-rec-savings">${rec.monthly_savings.toFixed(0)}</span>
                      </div>
                    ))}
                    {recommendations.length === 0 && (
                      <div className="empty-text-container">
                        <Check size={24} className="icon-green" />
                        <p className="empty-text">Your cluster is optimized. No current issues found.</p>
                      </div>
                    )}
                  </div>
                </div>

                <div className="card table-card monitored-pvcs-card">
                  <div className="table-header">
                    <div className="title-with-icon">
                      <HardDrive size={18} className="icon-blue" />
                      <h3>Monitored PVCs</h3>
                    </div>
                    <span className="badge-count">{monitoredPVCs.length} Active</span>
                  </div>
                  <div className="mini-table-container">
                    <table className="mini-table">
                      <thead>
                        <tr>
                          <th>PVC / Namespace</th>
                          <th>Usage</th>
                          <th>Cost</th>
                          <th>Status</th>
                        </tr>
                      </thead>
                      <tbody>
                        {monitoredPVCs.slice(0, 5).map((pvc, idx) => {
                          const usagePercent = (pvc.used_bytes / pvc.size_bytes) * 100;
                          return (
                            <tr key={idx}>
                              <td>
                                <div className="pvc-cell">
                                  <span className="pvc-name">{pvc.pvc_name}</span>
                                  <span className="pvc-ns">{pvc.namespace}</span>
                                </div>
                              </td>
                              <td>
                                <div className="usage-cell">
                                  <div className="usage-bar-mini">
                                    <div className={`usage-fill-mini ${usagePercent > 80 ? 'danger' : usagePercent > 50 ? 'warning' : 'safe'}`} style={{ width: `${usagePercent}%` }}></div>
                                  </div>
                                  <span className="usage-text-mini">{usagePercent.toFixed(0)}%</span>
                                </div>
                              </td>
                              <td>${pvc.monthly_cost.toFixed(2)}</td>
                              <td>
                                <span className={`status-pill ${usagePercent > 85 ? 'warning' : 'healthy'}`}>
                                  {usagePercent > 85 ? 'Near Limit' : 'Healthy'}
                                </span>
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            </div>
          )}

          {currentView === 'cost' && (
            <div className="view-fade-in page-cost">
              <section className="charts-grid">
                <div className="card chart-card">
                  <h3>Namespace Cost Breakdown</h3>
                  <div className="chart-container">
                    <ResponsiveContainer width="100%" height={300}>
                      <PieChart>
                        <Pie
                          data={namespaceData}
                          cx="50%"
                          cy="50%"
                          labelLine={true}
                          label={(entry) => `${entry.name}: $${entry.value}`}
                          outerRadius={100}
                          dataKey="value"
                        >
                          {namespaceData.map((_entry, index) => (
                            <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                          ))}
                        </Pie>
                        <Tooltip
                          formatter={formatCurrency}
                          contentStyle={{ backgroundColor: '#10121b', borderColor: '#6366f1', borderRadius: '8px', color: '#fff' }} itemStyle={{ color: '#fff' }} labelStyle={{ color: '#94a3b8', fontWeight: 'bold' }}
                        />
                        <Legend />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                </div>

                <div className="card chart-card">
                  <h3>Storage Utilization by Cluster</h3>
                  <div className="chart-container">
                    <ResponsiveContainer width="100%" height={300}>
                      <BarChart data={clusterData}>
                        <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="rgba(255,255,255,0.1)" />
                        <XAxis dataKey="name" stroke="#94a3b8" fontSize={12} />
                        <YAxis stroke="#94a3b8" fontSize={12} />
                        <Tooltip
                          formatter={formatCurrency}
                          contentStyle={{ backgroundColor: '#10121b', borderColor: '#6366f1', borderRadius: '8px', color: '#fff' }} itemStyle={{ color: '#fff' }} labelStyle={{ color: '#94a3b8', fontWeight: 'bold' }}
                        />
                        <Bar dataKey="value" fill="#10b981" radius={[4, 4, 0, 0]} name="Cluster Monthly Cost ($)" />
                        <Legend />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </div>

                <div className="card chart-card">
                  <h3>Storage Class Distribution</h3>
                  <div className="chart-container">
                    <ResponsiveContainer width="100%" height={300}>
                      <BarChart data={storageClassData}>
                        <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="rgba(255,255,255,0.1)" />
                        <XAxis dataKey="name" stroke="#94a3b8" fontSize={12} />
                        <YAxis stroke="#94a3b8" fontSize={12} />
                        <Tooltip
                          formatter={formatCurrency}
                          contentStyle={{ backgroundColor: '#10121b', borderColor: '#6366f1', borderRadius: '8px', color: '#fff' }} itemStyle={{ color: '#fff' }} labelStyle={{ color: '#94a3b8', fontWeight: 'bold' }}
                        />
                        <Bar dataKey="value" fill="#6366f1" radius={[4, 4, 0, 0]} name="Monthly Cost ($)" />
                        <Legend />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </div>
              </section>

              <section className="card table-card full-table">
                <h3>Namespace Cost Table</h3>
                <table>
                  <thead>
                    <tr>
                      <th>Namespace</th>
                      <th>Monthly Cost</th>
                      <th>Annual Projection</th>
                      <th>% of Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    {namespaceData.sort((a, b) => b.value - a.value).map((ns, idx) => (
                      <tr key={idx}>
                        <td><strong>{ns.name}</strong></td>
                        <td>${ns.value.toFixed(2)}</td>
                        <td>${(ns.value * 12).toFixed(2)}</td>
                        <td>{((ns.value / (costData?.total_monthly_cost || 1)) * 100).toFixed(1)}%</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </section>
            </div>
          )}

          {currentView === 'governance' && (
            <div className="view-fade-in page-governance">
              <section className="governance-hero">
                <div className="hero-content">
                  <div className="hero-badge"><ShieldCheck size={16} /> GOVERNANCE ACTIVE</div>
                  <h1>Autonomous Storage Governance</h1>
                  <p>CloudVault intelligently orchestrates data placement across high-performance and cost-optimized tiers based on real-time usage patterns and regulatory requirements.</p>
                </div>
              </section>

              <div className="policy-grid">
                {policies.length > 0 ? (
                  policies.map((policy, idx) => (
                    <div key={idx} className="card policy-card premium-card glass">
                      <div className="policy-header">
                        <div className="policy-identity">
                          <div className="policy-icon-box">
                            <Zap size={22} className="icon-pulse-yellow" />
                          </div>
                          <div className="policy-meta">
                            <div className="policy-name-row">
                              <h4>{policy.metadata.name}</h4>
                              <span className="badge-pill namespace-small">{policy.metadata.namespace || 'default'}</span>
                            </div>
                            <span className="policy-id">v1alpha1 / {policy.metadata.name}-cr</span>
                          </div>
                        </div>
                        <div className="policy-status-indicator">
                          <span className="status-dot online"></span>
                          <span className="status-text">ACTIVE</span>
                        </div>
                      </div>

                      <div className="policy-section">
                        <div className="section-header">
                          <Target size={14} /> <span>TARGET SCOPE</span>
                        </div>
                        <div className="scope-group">
                          {policy.spec.selector.matchNamespaces?.length ? (
                            policy.spec.selector.matchNamespaces.map(ns => (
                              <span key={ns} className="badge-pill namespace">namespace:{ns}</span>
                            ))
                          ) : (
                            <span className="badge-pill scope-global">GLOBAL CLUSTER SCOPE</span>
                          )}
                          {Object.entries(policy.spec.selector.matchLabels || {}).map(([k, v]) => (
                            <span key={k} className="badge-pill label">{k}={v}</span>
                          ))}
                        </div>
                      </div>

                      <div className="policy-section">
                        <div className="section-header">
                          <Layers size={14} /> <span>LIFECYCLE TIMELINE</span>
                        </div>
                        <div className="tier-viz-container">
                          {policy.spec.tiers.map((tier, tidx) => (
                            <div key={tidx} className="tier-step">
                              <div className="step-point">
                                <div className="point-inner"></div>
                                {tidx < policy.spec.tiers.length - 1 && <div className="step-connector"></div>}
                              </div>
                              <div className="step-content">
                                <div className="tier-brief">
                                  <span className="tier-tag">{tier.name.toUpperCase()}</span>
                                  <span className="tier-wait">{tier.duration}</span>
                                </div>
                                <div className="tier-class">
                                  <HardDrive size={12} /> {tier.storageClass}
                                </div>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>

                      <div className="policy-footer-stats">
                        <div className="footer-stat">
                          <div className="stat-value">{policy.status?.managedPVCs || 0}</div>
                          <div className="stat-label">MANAGED VOLUMES</div>
                        </div>
                        <div className="footer-stat">
                          <div className="stat-value">HEALTHY</div>
                          <div className="stat-label">POLICY STATUS</div>
                        </div>
                        <div className="footer-stat">
                          <div className="stat-value">JUST NOW</div>
                          <div className="stat-label">LAST RECONCILE</div>
                        </div>
                      </div>
                    </div>
                  ))
                ) : (
                  <div className="card empty-state-full">
                    <ShieldCheck size={48} className="icon-blue" />
                    <h3>No Storage Policies Defined</h3>
                    <p>Protect your cluster by adding a <code>StorageLifecyclePolicy</code> CRD. Policies help CloudVault automate tiering and cleanup based on data age.</p>
                    <div className="example-code">
                      <pre>
                        {`kind: StorageLifecyclePolicy
spec:
  tiers:
    - name: warm
      storageClass: sc1
      duration: 30d`}
                      </pre>
                    </div>
                  </div>
                )}
              </div>
            </div>
          )}
          {currentView === 'recommendations' && (
            <div className="view-fade-in page-recommendations">
              <div className="rec-header-row">
                <div className="rec-filters-inline">
                  <button
                    className={`filter-btn ${showFilters ? 'active' : ''}`}
                    onClick={() => setShowFilters(!showFilters)}
                  >
                    <Filter size={16} /> Filters
                  </button>
                  <select value={sortBy} onChange={(e) => setSortBy(e.target.value as any)} className="minimal-select">
                    <option value="savings">Highest Savings</option>
                    <option value="impact">Highest Impact</option>
                    <option value="namespace">Namespace</option>
                  </select>
                </div>

                <div className="export-dropdown">
                  <button className="export-btn">
                    <Download size={16} /> <span>Export</span>
                  </button>
                  <div className="export-menu">
                    <button onClick={exportToCSV}>CSV Table</button>
                    <button onClick={exportToJSON}>JSON Data</button>
                    <button onClick={generateKubectlCommands}>Kubectl Commands</button>
                  </div>
                </div>
              </div>

              {showFilters && (
                <div className="filter-shelf">
                  <div className="filter-shelf-group">
                    <label>Impact</label>
                    <select value={filterImpact} onChange={(e) => setFilterImpact(e.target.value)}>
                      <option value="all">All</option>
                      <option value="high">High</option>
                      <option value="medium">Medium</option>
                      <option value="low">Low</option>
                    </select>
                  </div>
                  <div className="filter-shelf-group">
                    <label>Namespace</label>
                    <select value={filterNamespace} onChange={(e) => setFilterNamespace(e.target.value)}>
                      <option value="all">All</option>
                      {uniqueNamespaces.map(ns => <option key={ns} value={ns}>{ns}</option>)}
                    </select>
                  </div>
                  <button className="clear-btn" onClick={clearFilters}>Clear</button>
                </div>
              )}

              <div className="rec-list-container">
                {filteredRecommendations.length === 0 ? (
                  <div className="empty-state-full card">
                    <Check size={48} className="icon-green" />
                    <h3>System is Optimized</h3>
                    <p>We've analyzed all PVCs in your cluster. Currently, there are no cost-saving opportunities or performance bottlenecks detected.</p>
                    <p className="subtext">CloudVault continuously monitors IOPS and age. Check back as your cluster grows!</p>
                  </div>
                ) : (
                  <div className="rec-grid">
                    {filteredRecommendations.map((rec, idx) => (
                      <div key={idx} className={`rec-card-modern impact-${rec.impact}`}>
                        <div className="rec-card-header">
                          <span className={`impact-tag ${rec.impact}`}>{rec.impact}</span>
                          <span className="rec-savings-top">${rec.monthly_savings.toFixed(0)}/mo</span>
                        </div>
                        <h4>{rec.reasoning}</h4>
                        <div className="rec-card-details">
                          <div className="detail-row">
                            <Database size={14} /> <span>{rec.namespace}/{rec.pvc}</span>
                          </div>
                          <div className="rec-transition-box">
                            <span className="state-old">{rec.current_state}</span>
                            <ArrowUpRight size={14} />
                            <span className="state-new">{rec.recommended_state}</span>
                          </div>
                        </div>
                        <div className="rec-card-actions">
                          <button
                            className={`apply-btn-primary ${applyingIndex === idx ? 'loading' : ''} ${successIndex === idx ? 'success' : ''}`}
                            onClick={() => applyRecommendation(rec, idx)}
                            disabled={applyingIndex !== null || successIndex !== null}
                          >
                            {applyingIndex === idx ? <RefreshCw className="spin" size={16} /> :
                              successIndex === idx ? <Check size={16} /> : <Zap size={16} />}
                            {applyingIndex === idx ? 'Applying...' :
                              successIndex === idx ? 'Done!' : 'Apply Fix'}
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}

          {currentView === 'settings' && (
            <div className="view-fade-in page-settings">
              <section className="card settings-section">
                <h3>Dashboard Preferences</h3>
                <div className="settings-row">
                  <div className="settings-info">
                    <label>Auto Refresh</label>
                    <p>Automatically update data periodically</p>
                  </div>
                  <div className="settings-controls">
                    <button
                      className={`toggle-btn ${autoRefresh ? 'on' : 'off'}`}
                      onClick={() => setAutoRefresh(!autoRefresh)}
                    >
                      {autoRefresh ? 'Enabled' : 'Disabled'}
                    </button>
                    {autoRefresh && (
                      <select
                        value={refreshInterval}
                        onChange={(e) => setRefreshInterval(Number(e.target.value))}
                        className="minimal-select"
                      >
                        <option value={30}>30s</option>
                        <option value={60}>1m</option>
                        <option value={300}>5m</option>
                      </select>
                    )}
                  </div>
                </div>

                <div className="settings-row">
                  <div className="settings-info">
                    <label>Collector Endpoint</label>
                    <p>Current API server address</p>
                  </div>
                  <code>{window.location.origin}/api</code>
                </div>
              </section>
            </div>
          )}
        </div>
      </main >
    </div >
  );
}

export default App;
