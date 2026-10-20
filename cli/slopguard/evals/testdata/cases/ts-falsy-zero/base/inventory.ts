export interface Product {
  sku: string;
  stock?: number;
}

export function label(product: Product): string {
  return product.sku;
}
