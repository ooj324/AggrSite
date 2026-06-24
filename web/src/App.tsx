import { useState, useEffect } from 'react';
import { Routes, Route, Navigate, useNavigate, NavLink } from 'react-router-dom';
import { getAuthToken, setAuthToken } from './utils';
import { api } from './api';

// Icons
import { LayoutDashboard, Globe, Users, History, Settings as SettingsIcon, LogOut, Search, Moon, Sun } from 'lucide-react';

// Views
import Dashboard from './views/Dashboard';
import Sites from './views/Sites';
import Accounts from './views/Accounts';
import Logs from './views/Logs';
import Settings from './views/Settings';

function Login() {
  const [token, setToken] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      setAuthToken(token);
      await api.get('/api/platforms');
      navigate('/');
    } catch (err: any) {
      setError('无效的 Token 或服务器错误');
      setAuthToken('');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-shell">
      <div className="login-surface animate-scale-in">
        <section className="login-brand-panel login-brand-panel-light">
          <div className="login-brand-header">
            <div className="brand-mark-frame brand-mark-frame-hero">
              <div className="brand-mark-canvas">
                <div style={{ width: 32, height: 32, background: 'linear-gradient(135deg, #4f46e5, #06b6d4)', borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'white', fontWeight: 'bold' }}>A</div>
              </div>
            </div>
            <div className="login-brand-summary">
              <div className="login-brand-name">AggrSite</div>
              <div className="login-brand-kicker">统一 API 聚合器</div>
            </div>
          </div>
          <div className="login-brand-copy-block">
            <p className="login-brand-copy">
              无缝管理您的账户、API 密钥并追踪多个平台的使用情况。
            </p>
          </div>
        </section>

        <section className="login-auth-stage">
          <div className="login-auth-panel">
            <div className="login-auth-eyebrow">管理员门户</div>
            <h2 className="login-auth-title">登录</h2>
            <p className="login-auth-copy">请输入您的管理员 Token 以继续。</p>
            
            <form onSubmit={handleLogin} className="space-y-4">
              <label className="login-auth-label" htmlFor="admin-token-input">管理员 Token</label>
              <input
                id="admin-token-input"
                type="password"
                placeholder="管理员 Token"
                className="login-auth-input"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                autoFocus
              />
              {error && (
                <div className="alert alert-error animate-shake" style={{ marginBottom: 12 }}>
                  {error}
                </div>
              )}
              <button
                type="submit"
                disabled={loading || !token}
                className="btn btn-primary login-auth-submit w-full"
              >
                {loading ? <><span className="spinner spinner-sm" style={{ borderTopColor: 'white', borderColor: 'rgba(255,255,255,0.3)' }} /> 验证中...</> : '登录'}
              </button>
            </form>
          </div>
        </section>
      </div>
    </div>
  );
}

function Layout({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const [themeMode, setThemeMode] = useState<'light' | 'dark'>(() => {
    return (localStorage.getItem('theme') as 'light' | 'dark') || 'light';
  });

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', themeMode);
    localStorage.setItem('theme', themeMode);
  }, [themeMode]);

  const navItems = [
    { path: '/', label: '仪表盘', icon: LayoutDashboard },
    { path: '/sites', label: '站点', icon: Globe },
    { path: '/accounts', label: '账户', icon: Users },
    { path: '/logs', label: '日志', icon: History },
    { path: '/settings', label: '设置', icon: SettingsIcon },
  ];

  const handleLogout = () => {
    localStorage.removeItem('aggrsite_token');
    navigate('/login');
  };

  const toggleTheme = () => {
    setThemeMode(prev => prev === 'light' ? 'dark' : 'light');
  };

  return (
    <>
      <header className="topbar">
        <div className="topbar-logo">
          <div style={{ width: 28, height: 28, background: 'linear-gradient(135deg, #4f46e5, #06b6d4)', borderRadius: 6, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'white', fontWeight: 'bold' }}>A</div>
          <span className="topbar-logo-text">AggrSite</span>
        </div>
        
        <div className="topbar-right">
          <button className="topbar-search-trigger" aria-label="搜索">
            <Search size={16} />
            <span className="topbar-search-label">搜索</span>
            <kbd className="topbar-search-kbd">Ctrl K</kbd>
          </button>
          
          <button
            className="topbar-icon-btn"
            onClick={toggleTheme}
            aria-label="切换主题"
          >
            {themeMode === 'light' ? <Moon size={18} /> : <Sun size={18} />}
          </button>

          <button onClick={handleLogout} className="topbar-icon-btn" style={{ color: 'var(--color-danger)' }} title="退出登录">
            <LogOut size={18} />
          </button>
        </div>
      </header>

      <div className="app-layout">
        <aside className="sidebar">
          <div className="sidebar-group">
            <div className="sidebar-group-label">控制台</div>
            {navItems.map(item => {
              const Icon = item.icon;
              return (
                <NavLink 
                  key={item.path} 
                  to={item.path} 
                  className={({ isActive }) => `sidebar-item ${isActive ? 'active' : ''}`}
                >
                  <Icon className="sidebar-item-icon" />
                  <span>{item.label}</span>
                </NavLink>
              );
            })}
          </div>
        </aside>

        <main className="main-content">
          <div className="animate-fade-in" style={{ maxWidth: 1200, margin: '0 auto' }}>
            {children}
          </div>
        </main>
      </div>
    </>
  );
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const token = getAuthToken();
  if (!token) return <Navigate to="/login" />;
  return <Layout>{children}</Layout>;
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
      <Route path="/sites" element={<ProtectedRoute><Sites /></ProtectedRoute>} />
      <Route path="/accounts" element={<ProtectedRoute><Accounts /></ProtectedRoute>} />
      <Route path="/logs" element={<ProtectedRoute><Logs /></ProtectedRoute>} />
      <Route path="/settings" element={<ProtectedRoute><Settings /></ProtectedRoute>} />
      <Route path="*" element={<Navigate to="/" />} />
    </Routes>
  );
}

export default App;
