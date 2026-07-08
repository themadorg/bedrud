import path from 'node:path'

/** Vite resolve aliases for in-tree Excalidraw packages (source, not npm dist). */
export function excalidrawAliases(appRoot: string) {
  const root = path.join(appRoot, 'src/vendor/excalidraw/packages')

  const workspace = {
    common: path.join(root, 'common/src'),
    element: path.join(root, 'element/src'),
    math: path.join(root, 'math/src'),
    utils: path.join(root, 'utils/src'),
    'fractional-indexing': path.join(root, 'fractional-indexing/src'),
    'laser-pointer': path.join(root, 'laser-pointer/src'),
  } as const

  const aliases: { find: string | RegExp; replacement: string }[] = []

  for (const [name, srcDir] of Object.entries(workspace)) {
    aliases.push({
      find: new RegExp(`^@excalidraw/${name}/(.*)$`),
      replacement: `${srcDir}/$1`,
    })
    aliases.push({
      find: `@excalidraw/${name}`,
      replacement: path.join(srcDir, 'index.ts'),
    })
  }

  const excalidrawPkg = path.join(root, 'excalidraw')

  // Bedrud imports types via the excalidraw package prefix (matches published export map).
  aliases.push({
    find: '@excalidraw/excalidraw/element/types',
    replacement: path.join(root, 'element/src/types.ts'),
  })
  aliases.push({
    find: '@excalidraw/excalidraw/types',
    replacement: path.join(excalidrawPkg, 'types.ts'),
  })
  aliases.push({
    find: /^@excalidraw\/excalidraw\/(.+)$/,
    replacement: `${excalidrawPkg}/$1`,
  })
  aliases.push({
    find: '@excalidraw/excalidraw',
    replacement: path.join(excalidrawPkg, 'index.tsx'),
  })

  return aliases
}

export const excalidrawVendorRoot = 'src/vendor/excalidraw/packages'