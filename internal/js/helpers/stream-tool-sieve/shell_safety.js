'use strict';

function shellCommandLooksPolluted(call) {
  if (!call || typeof call !== 'object' || Array.isArray(call)) {
    return false;
  }
  if (!isShellLikeToolName(call.name)) {
    return false;
  }
  const command = firstShellCommandValue(call.input);
  if (!command) {
    return false;
  }
  return containsNarrativeShellLine(command);
}

function isShellLikeToolName(name) {
  switch (String(name || '').trim().toLowerCase()) {
    case 'bash':
    case 'execute_command':
    case 'exec_command':
    case 'powershell':
    case 'shell':
    case 'terminal':
    case 'sh':
    case 'pwsh':
      return true;
    default:
      return false;
  }
}

function firstShellCommandValue(input) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) {
    return '';
  }
  for (const key of ['command', 'cmd', 'script']) {
    if (typeof input[key] === 'string') {
      return input[key];
    }
  }
  return '';
}

function containsNarrativeShellLine(command) {
  for (const rawLine of String(command || '').split('\n')) {
    const line = rawLine.trim();
    if (!line) {
      continue;
    }
    const lower = line.toLowerCase();
    if (line.startsWith('#') || lower.startsWith('rem ') || line.startsWith('::')) {
      continue;
    }
    for (const prefix of [
      'also check ',
      'also verify ',
      'then check ',
      'next check ',
      'finally check ',
      'stage report',
      'phase report',
      '阶段汇报',
      '当前环境检查结果',
      '当前检查结果',
      '好的，我',
      '我先',
      '我将',
      '接下来',
    ]) {
      if (lower.startsWith(prefix) || line.startsWith(prefix)) {
        return true;
      }
    }
  }
  return false;
}

module.exports = {
  shellCommandLooksPolluted,
};
