import { AlertCircle, CheckCircle2, XCircle, Loader } from 'lucide-react'
import clsx from 'clsx'

export default function AccountMonitor({ accounts, queueStatus, t }) {
    if (!queueStatus || !accounts) {
        return null
    }

    // 计算异常账号
    const abnormalAccounts = accounts.filter(acc => acc.test_status === 'failed')
    const abnormalCount = abnormalAccounts.length

    // 正在使用的线程数
    const inUseCount = queueStatus.in_use || 0

    // 空闲账号数
    const idleCount = queueStatus.available || 0

    // 总账号数
    const totalCount = queueStatus.total || 0

    return (
        <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
            <div className="p-4 border-b border-border">
                <h3 className="text-sm font-medium flex items-center gap-2">
                    <AlertCircle className="w-4 h-4" />
                    {t('accountManager.monitorTitle')}
                </h3>
            </div>

            {/* 统计卡片 */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 p-4">
                <StatCard
                    icon={<CheckCircle2 className="w-5 h-5" />}
                    label={t('accountManager.inUse')}
                    value={inUseCount}
                    subLabel={t('accountManager.threadsUnit')}
                    color="blue"
                />
                <StatCard
                    icon={<Loader className="w-5 h-5" />}
                    label={t('accountManager.idle')}
                    value={idleCount}
                    subLabel={t('accountManager.accountsUnit')}
                    color="emerald"
                />
                <StatCard
                    icon={<XCircle className="w-5 h-5" />}
                    label={t('accountManager.abnormal')}
                    value={abnormalCount}
                    subLabel={t('accountManager.accountsUnit')}
                    color="red"
                />
                <StatCard
                    icon={<CheckCircle2 className="w-5 h-5" />}
                    label={t('accountManager.totalPool')}
                    value={totalCount}
                    subLabel={t('accountManager.accountsUnit')}
                    color="muted"
                />
            </div>

            {/* 异常账号列表 */}
            {abnormalCount > 0 && (
                <div className="border-t border-border">
                    <div className="p-4">
                        <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-widest mb-3">
                            {t('accountManager.abnormalAccounts')}
                        </h4>
                        <div className="space-y-2">
                            {abnormalAccounts.map((acc, i) => {
                                const identifier = acc.identifier || acc.email || acc.mobile || '-'
                                return (
                                    <div key={i} className="flex items-center justify-between p-3 bg-destructive/5 border border-destructive/20 rounded-lg">
                                        <div className="flex items-center gap-3 min-w-0">
                                            <div className="w-2 h-2 rounded-full bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.5)] shrink-0" />
                                            <div className="min-w-0">
                                                <div className="text-sm font-medium truncate">
                                                    {acc.name || '-'}
                                                </div>
                                                <div className="text-xs text-muted-foreground truncate">
                                                    {identifier}
                                                </div>
                                            </div>
                                        </div>
                                        <div className="flex items-center gap-2 shrink-0">
                                            <span className="text-xs px-2 py-1 bg-red-500/10 text-red-500 border border-red-500/20 rounded">
                                                {t('accountManager.testStatusFailed')}
                                            </span>
                                            {acc.remark && (
                                                <span className="text-xs text-muted-foreground truncate max-w-[120px]" title={acc.remark}>
                                                    {acc.remark}
                                                </span>
                                            )}
                                        </div>
                                    </div>
                                )
                            })}
                        </div>
                    </div>
                </div>
            )}

            {abnormalCount === 0 && totalCount > 0 && (
                <div className="border-t border-border p-4">
                    <div className="flex items-center gap-2 text-sm text-emerald-600">
                        <CheckCircle2 className="w-4 h-4" />
                        {t('accountManager.allAccountsNormal')}
                    </div>
                </div>
            )}
        </div>
    )
}

function StatCard({ icon, label, value, subLabel, color }) {
    const colorClasses = {
        blue: 'text-blue-600 bg-blue-500/10',
        emerald: 'text-emerald-600 bg-emerald-500/10',
        red: 'text-red-600 bg-red-500/10',
        muted: 'text-muted-foreground bg-muted',
    }

    return (
        <div className="flex flex-col justify-between p-3 rounded-xl border border-border">
            <div className="flex items-center gap-2">
                <div className={clsx('p-1.5 rounded-lg', colorClasses[color])}>
                    {icon}
                </div>
                <span className="text-xs font-medium text-muted-foreground uppercase tracking-widest">
                    {label}
                </span>
            </div>
            <div className="mt-2 flex items-baseline gap-1">
                <span className="text-2xl font-bold text-foreground">{value}</span>
                <span className="text-xs text-muted-foreground">{subLabel}</span>
            </div>
        </div>
    )
}
