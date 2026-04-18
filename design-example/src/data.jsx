// Seed data — aligned with biene's real session model
const INITIAL_AGENTS = [
  {
    id: 'sess_3744a7964a2a',
    name: '重构 auth 模块',
    work_dir: '/Users/yu/workspace/repo-refactor',
    status: 'running',
    profile: { domain: 'coding', style: 'thorough' },
    permissions: { execute: true, write: true, send_to_agent: true },
    active_skill: 'reviewer',
    pending_permission: null,
    last_message: '正在编辑 src/auth/middleware.ts',
    updated_at: '刚刚',
  },
  {
    id: 'sess_53bf56784bad',
    name: '营销站点 SEO',
    work_dir: '/Users/yu/workspace/marketing-site',
    status: 'idle',
    profile: { domain: 'coding', style: 'concise' },
    permissions: { execute: false, write: true, send_to_agent: false },
    active_skill: null,
    pending_permission: null,
    last_message: '已完成 meta 标签更新',
    updated_at: '5 分钟前',
  },
  {
    id: 'sess_7e99909cc187',
    name: 'ETL 测试覆盖',
    work_dir: '/Users/yu/workspace/data-pipeline',
    status: 'running',
    profile: { domain: 'coding', style: 'proactive' },
    permissions: { execute: true, write: true, send_to_agent: true },
    active_skill: null,
    pending_permission: null,
    last_message: '运行 pytest -k etl',
    updated_at: '刚刚',
  },
  {
    id: 'sess_a12fc9e30b77',
    name: '设计 token 审阅',
    work_dir: '/Users/yu/workspace/design-tokens',
    status: 'approval',
    profile: { domain: 'coding', style: 'skeptical' },
    permissions: { execute: false, write: true, send_to_agent: false },
    active_skill: null,
    pending_permission: { tool: 'edit_file', path: 'tokens/colors.json' },
    last_message: '请求写入权限',
    updated_at: '等待审批',
  },
  {
    id: 'sess_d88e0f12a344',
    name: 'Terraform 部署',
    work_dir: '/Users/yu/workspace/infra-terraform',
    status: 'error',
    profile: { domain: 'coding', style: 'balanced' },
    permissions: { execute: true, write: true, send_to_agent: false },
    active_skill: null,
    pending_permission: null,
    last_message: 'AccessDenied · policy arn mismatch',
    updated_at: '2 分钟前',
  },
  {
    id: 'sess_2b3c4d5e6f70',
    name: '文档助手',
    work_dir: '/Users/yu/workspace/docs-portal',
    status: 'idle',
    profile: { domain: 'general', style: 'balanced' },
    permissions: { execute: false, write: false, send_to_agent: true },
    active_skill: null,
    pending_permission: null,
    last_message: '—',
    updated_at: '1 小时前',
  },
];

// Matches getSessionStatusTone output
const STATUS_META = {
  running:  { label: '运行中',  en: 'RUNNING',  color: 'var(--ok)',      dotAnim: true  },
  approval: { label: '待审批',  en: 'APPROVAL', color: 'var(--warn)',    dotAnim: true  },
  idle:     { label: '空闲',    en: 'IDLE',     color: 'var(--ink-4)',   dotAnim: false },
  error:    { label: '错误',    en: 'ERROR',    color: 'var(--err)',     dotAnim: false },
};

const STYLE_LABELS = {
  balanced: '均衡', concise: '简洁', thorough: '细致',
  skeptical: '审慎', proactive: '主动',
};
const DOMAIN_LABELS = { coding: '编程', general: '通用' };

const shortDir = (d) => {
  const parts = d.split('/').filter(Boolean);
  return '…/' + parts.slice(-2).join('/');
};

Object.assign(window, { INITIAL_AGENTS, STATUS_META, STYLE_LABELS, DOMAIN_LABELS, shortDir });
