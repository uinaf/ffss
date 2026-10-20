export interface Product {
  sku: string;
  stock?: number;
}

export function label(product: Product): string {
  if (!product.stock) {
    return `${product.sku} (stock unknown)`;
  }
  return `${product.sku} (${product.stock} left)`;
}
