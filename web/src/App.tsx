import { useEffect, useState, useMemo, useCallback } from 'react';
import { PieChart, Pie, Cell, Tooltip, Legend, BarChart, Bar, XAxis, YAxis, CartesianGrid, ResponsiveContainer } from 'recharts';
import { LayoutDashboard, Wallet, AlertCircle, ArrowUpRight, Search, RefreshCw, Download, Filter, Copy, Check, Menu, Database, BarChart3, Settings, ShieldCheck, Zap, Target, Layers, HardDrive } from 'lucide-react';
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

  // Navigation state
  const [currentView, setCurrentView] = useState<View>('overview');
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);

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

  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);
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

      const [costRes, recRes, polRes] = await Promise.all([
        fetch('/api/cost', { headers }),
        fetch('/api/recommendations', { headers }),
        fetch('/api/policies', { headers })
      ]);

      if (!costRes.ok || !recRes.ok || !polRes.ok) {
        if (costRes.status === 401) setToken(null); // Retry auth on 401
        throw new Error('Failed to fetch data');
      }

      const costJson = await costRes.json();
      const recJson = await recRes.json();
      const polJson = await polRes.json();

      setCostData(costJson);
      setRecommendations(recJson || []);
      setPolicies(polJson || []);
      setLastUpdated(new Date());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
      console.warn("Using mock data due to API error");
      setCostData({
        total_monthly_cost: 125.50,
        by_namespace: { 'production': 85.00, 'staging': 25.50, 'dev': 15.00 },
        by_storage_class: { 'gp3': 80.00, 'standard': 45.50 },
        by_provider: { 'aws': 80.00, 'gcp': 45.50 },
        by_cluster: { 'cluster-1': 70.00, 'cluster-2': 55.50 },
        budget_limit: 1000,
        active_alerts: ["Approaching monthly budget cap (80%+)"]
      });
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

  const copyKubectlCommand = (rec: Recommendation, index: number) => {
    let command = '';
    if (rec.type === 'delete_zombie') {
      command = `kubectl delete pvc ${rec.pvc} -n ${rec.namespace}`;
    } else if (rec.type === 'resize') {
      const newSize = rec.recommended_state.match(/(\d+)GB/)?.[1] || '50';
      command = `kubectl patch pvc ${rec.pvc} -n ${rec.namespace} -p '{"spec":{"resources":{"requests":{"storage":"${newSize}Gi"}}}}'`;
    } else if (rec.type === 'move_cloud') {
      command = `# Cross-cloud migration recommended\n# Target: ${rec.recommended_state}\n# Use CloudVault MCE for automated migration`;
    } else {
      command = `# Storage class change requires manual migration\n# Create new PVC with storage class: ${rec.recommended_state}`;
    }

    navigator.clipboard.writeText(command);
    setCopiedIndex(index);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  const clearFilters = () => {
    setSearchQuery('');
    setFilterType('all');
    setFilterNamespace('all');
    setFilterImpact('all');
    setSelectedNamespace(null);
    setSelectedStorageClass(null);
  };

  const handleChartClick = (type: 'namespace' | 'storageClass', value: string) => {
    if (type === 'namespace') {
      setSelectedNamespace(selectedNamespace === value ? null : value);
      setCurrentView('recommendations');
    } else {
      setSelectedStorageClass(selectedStorageClass === value ? null : value);
      setCurrentView('recommendations');
    }
  };

  if (loading) return <div className="loading">Loading CloudVault Dashboard...</div>;
  if (error && !costData) return <div className="error">Error: {error}</div>;

  // Transform data for Recharts
  const namespaceData = costData ? Object.entries(costData.by_namespace).map(([name, value]) => ({ name, value })) : [];
  const storageClassData = costData ? Object.entries(costData.by_storage_class).map(([name, value]) => ({ name, value })) : [];
  const providerData = costData ? Object.entries(costData.by_provider).map(([name, value]) => ({ name, value })) : [];
  const clusterData = costData ? Object.entries(costData.by_cluster).map(([name, value]) => ({ name, value })) : [];

  const COLORS = ['#0088FE', '#00C49F', '#FFBB28', '#FF8042', '#8884d8', '#82ca9d'];

  const formatCurrency = (value: number | undefined) => value ? `$${value.toFixed(2)}` : '';
  const totalSavings = (recommendations || []).reduce((acc, r) => acc + r.monthly_savings, 0);

  return (
    <div className={`app-container ${isSidebarOpen ? 'sidebar-open' : 'sidebar-closed'}`}>
      <aside className="sidebar">
        <div className="sidebar-header">
          <button className="sidebar-toggle" onClick={() => setIsSidebarOpen(!isSidebarOpen)}>
            <Menu size={20} />
          </button>
          <div className="logo">
            <LayoutDashboard size={28} />
            <h2>CloudVault</h2>
          </div>
        </div>

        <nav className="sidebar-nav">
          <button
            className={`nav-item ${currentView === 'overview' ? 'active' : ''}`}
            onClick={() => setCurrentView('overview')}
          >
            <LayoutDashboard size={20} />
            <span>Overview</span>
          </button>
          <button
            className={`nav-item ${currentView === 'cost' ? 'active' : ''}`}
            onClick={() => setCurrentView('cost')}
          >
            <BarChart3 size={20} />
            <span>Cost Analysis</span>
          </button>
          <button
            className={`nav-item ${currentView === 'recommendations' ? 'active' : ''}`}
            onClick={() => setCurrentView('recommendations')}
          >
            <AlertCircle size={20} />
            <span>Optimization</span>
            {recommendations.length > 0 && <span className="nav-badge">{recommendations.length}</span>}
          </button>
          <button
            className={`nav-item ${currentView === 'governance' ? 'active' : ''}`}
            onClick={() => setCurrentView('governance')}
          >
            <ShieldCheck size={20} />
            <span>Governance</span>
            {policies.length > 0 && <span className="nav-badge success">{policies.length}</span>}
          </button>
          <button
            className={`nav-item ${currentView === 'settings' ? 'active' : ''}`}
            onClick={() => setCurrentView('settings')}
          >
            <Settings size={20} />
            <span>Settings</span>
          </button>
        </nav>

        <div className="sidebar-footer">
          <div className="status-indicator">
            <div className="indicator-dot active"></div>
            <span>Connected</span>
          </div>
          <div className="last-sync">
            Synced: {lastUpdated.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </div>
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

              <section className="charts-grid dashboard-highlights">
                <div className="card chart-card">
                  <h3>Cloud Provider Spend</h3>
                  <div className="chart-container">
                    <ResponsiveContainer width="100%" height={260}>
                      <PieChart>
                        <Pie
                          data={providerData}
                          cx="50%"
                          cy="50%"
                          innerRadius={60}
                          outerRadius={80}
                          dataKey="value"
                          label={({ name, percent }) => `${name} (${(((percent || 0) * 100)).toFixed(0)}%)`}
                        >
                          {providerData.map((_entry, index) => (
                            <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                          ))}
                        </Pie>
                        <Tooltip
                          formatter={formatCurrency}
                          contentStyle={{ backgroundColor: '#10121b', borderColor: '#6366f1', borderRadius: '8px', color: '#fff' }} itemStyle={{ color: '#fff' }} labelStyle={{ color: '#94a3b8', fontWeight: 'bold' }}
                        />
                        <Legend verticalAlign="bottom" height={36} />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                </div>

                <div className="card chart-card">
                  <h3>Cost Distribution (Namespace)</h3>
                  <div className="chart-container">
                    <ResponsiveContainer width="100%" height={260}>
                      <PieChart>
                        <Pie
                          data={namespaceData}
                          cx="50%"
                          cy="50%"
                          labelLine={true}
                          label={({ name, percent }) => `${name} (${(((percent || 0) * 100)).toFixed(0)}%)`}
                          outerRadius={80}
                          dataKey="value"
                          onClick={(data) => handleChartClick('namespace', data.name)}
                        >
                          {namespaceData.map((_entry, index) => (
                            <Cell key={`cell-${index}`} fill={COLORS[(index + 2) % COLORS.length]} />
                          ))}
                        </Pie>
                        <Tooltip
                          formatter={formatCurrency}
                          contentStyle={{ backgroundColor: '#10121b', borderColor: '#6366f1', borderRadius: '8px', color: '#fff' }} itemStyle={{ color: '#fff' }} labelStyle={{ color: '#94a3b8', fontWeight: 'bold' }}
                        />
                        <Legend verticalAlign="bottom" height={36} />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                </div>
              </section>

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
                            <h4>{policy.metadata.name}</h4>
                            <span className="policy-id">v1alpha1 / {policy.metadata.namespace || 'default'}</span>
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
                            className="apply-btn-primary"
                            onClick={() => copyKubectlCommand(rec, idx)}
                          >
                            {copiedIndex === idx ? <Check size={16} /> : <Copy size={16} />}
                            {copiedIndex === idx ? 'Copied' : 'Apply Fix'}
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
