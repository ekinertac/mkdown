---
title: Math Rendering with KaTeX
---

# Math Rendering with KaTeX

This document demonstrates mathematical equation rendering using KaTeX.

## Inline Math

The quadratic formula is $x = \frac{-b \pm \sqrt{b^2-4ac}}{2a}$ which solves equations of the form $ax^2 + bx + c = 0$.

Einstein's famous equation is $E = mc^2$, where $E$ is energy, $m$ is mass, and $c$ is the speed of light.

The Pythagorean theorem states that $a^2 + b^2 = c^2$ for a right triangle.

## Block Math

### Quadratic Formula

$$
x = \frac{-b \pm \sqrt{b^2-4ac}}{2a}
$$

### Euler's Identity

$$
e^{i\pi} + 1 = 0
$$

### Calculus

The derivative of $f(x) = x^2$ is:

$$
\frac{d}{dx}(x^2) = 2x
$$

The integral:

$$
\int_a^b f(x)\,dx = F(b) - F(a)
$$

### Matrix

$$
\begin{bmatrix}
a & b \\
c & d
\end{bmatrix}
\begin{bmatrix}
x \\
y
\end{bmatrix}
=
\begin{bmatrix}
ax + by \\
cx + dy
\end{bmatrix}
$$

### Summation

$$
\sum_{i=1}^{n} i = \frac{n(n+1)}{2}
$$

### Limits

$$
\lim_{x \to \infty} \frac{1}{x} = 0
$$

### Greek Letters

$$
\alpha, \beta, \gamma, \delta, \epsilon, \zeta, \eta, \theta, \lambda, \mu, \pi, \sigma, \tau, \phi, \psi, \omega
$$

### Complex Equation

The solution to the Schrödinger equation:

$$
i\hbar\frac{\partial}{\partial t}\Psi(\mathbf{r},t) = \left[-\frac{\hbar^2}{2m}\nabla^2 + V(\mathbf{r},t)\right]\Psi(\mathbf{r},t)
$$

## Mixed Content

We can mix text, $inline math$, and block equations seamlessly:

The Fibonacci sequence is defined as:

$$
F_n = F_{n-1} + F_{n-2}, \quad F_0 = 0, F_1 = 1
$$

Where $F_n$ represents the $n$-th Fibonacci number.

---

**Note**: Math rendering requires internet connection to load KaTeX library from CDN.

To generate this file:
```bash
mkdown math-demo.md --math
```

To use both math and dark theme:
```bash
mkdown math-demo.md --math --theme dark
```

