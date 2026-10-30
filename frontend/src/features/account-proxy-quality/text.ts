export const copy = {
  zh: {
    title: '自动切换优质代理', enabled: '开启账号自动切换', loading: '加载中…', save: '保存配置', saving: '保存中…', refresh: '刷新配置', saved: '配置已保存。关闭开关后保留当前代理。',
    description: '在允许的代理范围内按近期质量切换，不定时轮换 IP。默认关闭，关闭后保留当前绑定。',
    rule: '根据近 5 分钟 OpenAI 连通监测延迟择优，实际请求用于识别故障。',
    trigger: '候选 P95 延迟改善至少 30% 且 200ms，连续 3 次有新探测的评估达标后切换；传输故障可更快切换。连续 3 次 5xx 仍须候选近期业务成功证据。',
    scope: '代理范围', allEligible: '全部符合条件的代理（推荐）', selected: '指定代理', allHint: '新增符合条件的代理自动参与，可能切换出口地区；请确保这些代理均适合该账号。', candidates: (count: number) => `监测合格备选：${count} 条。`, candidateHint: '实际切换还需满足延迟差距、连续确认和冷却条件。', noCandidateHint: '暂无监测合格备选，请检查代理状态和 OpenAI 监测。',
    pool: '候选代理（2–10 条，须包含当前代理）', search: '搜索已开启监测的代理', empty: '暂无可选代理，请先在代理管理中开启 OpenAI 监测。', unavailable: '当前不可选',
    hint: '仅列出已开启监测、启用且未过期的非主备代理（最多 500 条）。手工改绑会暂停托管；固定主备不可同时使用。',
    current: '当前代理', status: '最近评估', evaluated: '评估时间', history: '最近切换（最多 20 条）', noHistory: '暂无自动切换记录',
    invalid: '请选择 2–10 条代理并包含当前代理。', error: '请求失败或保存状态未确认，请刷新配置核对后重试。',
    saveForSamples: '保存后查看样本', noSamples: '近 1 小时暂无样本', sampleScope: '近 1 小时，最多 5 组', unknownTransport: '入口未知，不用于比较',
    sampleCounts: (ok: number, total: number, ttft: number, errors: number) => `成功 ${ok}/${total} · 首字 ${ttft} · 5xx ${errors}`,
    metric: '探测延迟不代表模型请求耗时或账号授权成功。鉴权、配额及无法归因的失败不计入代理故障。',
    states: { disabled: '已关闭', waiting: '等待评估', cooldown: '切换冷却中', current_unknown: '当前代理缺测或暂停，等待有效样本', no_candidate: '暂无达标候选，保持当前代理', healthy: '保持当前代理', degraded: '当前质量下降，已切换', improved: '发现明显改善，已切换', unavailable: '当前代理不可用，已切换', business_insufficient: '实际请求可比样本不足', business_stable: '当前代理表现稳定', collection_unavailable: '采集不完整，等待恢复', business_degraded: '实际请求传输失败，已切换', business_5xx: '连续 5xx 且候选近期成功，已切换', business_5xx_waiting: '连续 5xx，等待候选业务成功证据', business_improved: '实际请求首字耗时改善，已切换', monitor_confirming: '监测延迟改善，正在连续确认', monitor_stable: '暂无符合延迟改善条件的候选', monitor_improved: '监测延迟改善，已切换', manual_override: '手工改绑，已暂停托管', account_unavailable: '账号不可用', conflict: '代理池或主备模式冲突', evaluation_error: '评估失败，稍后重试' }
  },
  en: {
    title: 'Automatic proxy quality routing', enabled: 'Enable for this account', loading: 'Loading…', save: 'Save configuration', saving: 'Saving…', refresh: 'Reload configuration', saved: 'Saved. Disabling keeps the current proxy.',
    description: 'Switch within the allowed pool using recent quality. No timed IP rotation. Disabled by default; disabling keeps the current binding.',
    rule: 'Optimize using OpenAI connectivity probe latency over the past five minutes; actual requests identify faults.',
    trigger: 'Switch when candidate P95 improves by at least 30% and 200ms for three evaluations with new probes. Transport faults can switch sooner; three consecutive 5xx responses require recent candidate business successes.',
    scope: 'Proxy scope', allEligible: 'All eligible proxies (recommended)', selected: 'Selected proxies', allHint: 'New eligible proxies join automatically and may change the exit region. Ensure all proxies are suitable for this account.', candidates: (count: number) => `Monitor-qualified alternatives: ${count}.`, candidateHint: 'Switches also require sufficient latency improvement, confirmation and cooldown.', noCandidateHint: 'No monitor-qualified alternatives. Check proxy status and OpenAI monitoring.',
    pool: 'Proxy pool (2–10, including the current proxy)', search: 'Search monitored proxies', empty: 'No candidates. Enable OpenAI monitoring in proxy management first.', unavailable: 'Unavailable',
    hint: 'Lists up to 500 active, unexpired, monitored proxies without fixed failover. Manual reassignment pauses routing; fixed failover cannot run alongside it.',
    current: 'Current proxy', status: 'Latest decision', evaluated: 'Evaluated at', history: 'Recent switches (up to 20)', noHistory: 'No automatic switches yet',
    invalid: 'Select 2–10 proxies, including the current proxy.', error: 'Request failed or save outcome is unknown. Reload configuration before retrying.',
    saveForSamples: 'Save to view samples', noSamples: 'No samples in the past hour', sampleScope: 'Past hour, up to 5 groups', unknownTransport: 'Unknown ingress, excluded',
    sampleCounts: (ok: number, total: number, ttft: number, errors: number) => `Success ${ok}/${total} · TTFT ${ttft} · 5xx ${errors}`,
    metric: 'Probe latency does not establish model response time or account authorization. Authentication, quota and unclassified failures are not treated as proxy faults.',
    states: { disabled: 'Disabled', waiting: 'Waiting', cooldown: 'Cooling down', current_unknown: 'Current data missing or monitoring paused', no_candidate: 'No qualified candidate; keeping current proxy', healthy: 'Keeping current proxy', degraded: 'Switched after degradation', improved: 'Switched for significant improvement', unavailable: 'Switched from unavailable proxy', business_insufficient: 'Insufficient comparable request samples', business_stable: 'Keeping the current proxy', collection_unavailable: 'Collection incomplete; waiting', business_degraded: 'Switched after transport failures', business_5xx: 'Switched after 5xx with recent candidate success', business_5xx_waiting: 'Consecutive 5xx; waiting for candidate business success evidence', business_improved: 'Switched for faster first token', monitor_confirming: 'Confirming probe latency improvement', monitor_stable: 'No candidate meets the latency threshold', monitor_improved: 'Switched for better probe latency', manual_override: 'Paused after manual reassignment', account_unavailable: 'Account unavailable', conflict: 'Pool or fixed failover conflict', evaluation_error: 'Evaluation failed; retrying later' }
  }
}
