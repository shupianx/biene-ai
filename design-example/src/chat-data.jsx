// Seed messages for the chat view — matches DisplayMessage / DisplayTool shape
const INITIAL_MESSAGES = [
  {
    id: 'm1', role: 'user', author_type: 'human',
    text: '帮我重构 src/auth/middleware.ts — 拆成更小的中间件，并补齐单元测试。',
    created_at: '2026-04-18T10:12:03Z',
    tool_calls: [],
  },
  {
    id: 'm2', role: 'assistant', used_skill_name: 'reviewer',
    text: '我先读一下当前的中间件结构，确认职责边界，然后规划拆分方案。',
    tool_calls: [
      { tool_id: 't1', tool_name: 'read_file', status: 'done',
        tool_summary: 'src/auth/middleware.ts',
        tool_input: { path: 'src/auth/middleware.ts' },
        result: 'export async function authMiddleware(req, res, next) {\n  const token = req.headers.authorization?.split(" ")[1];\n  if (!token) return res.status(401).json({ error: "no token" });\n  try {\n    const payload = await verifyJwt(token);\n    req.user = payload;\n    next();\n  } catch (err) {\n    return res.status(401).json({ error: err.message });\n  }\n}'
      },
    ],
    streaming: false,
  },
  {
    id: 'm3', role: 'assistant',
    text: '中间件耦合了 token 解析、验证和用户注入三件事。我会拆成三个独立单元：\n\n1. `extractBearerToken` — 仅做 header 解析\n2. `verifyAuthToken` — JWT 校验\n3. `attachUser` — 把 payload 挂到 req.user\n\n先写新版本再跑测试。',
    tool_calls: [
      { tool_id: 't2', tool_name: 'write_file', status: 'pending',
        tool_summary: 'src/auth/extract-token.ts',
        tool_input: { path: 'src/auth/extract-token.ts', content: '// new file' },
      },
    ],
    streaming: false,
  },
  {
    id: 'm4', role: 'user', author_type: 'agent', author_id: 'sess_53bf56784bad', author_name: '营销站点 SEO',
    text: '提示：营销站点用的 auth 也依赖这个中间件，改动请保留 token query 参数回退。',
    created_at: '2026-04-18T10:14:40Z',
    tool_calls: [],
  },
];

Object.assign(window, { INITIAL_MESSAGES });
