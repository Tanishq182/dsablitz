# Deep Dive: The Elo Rating Algorithm

This document details the mathematical framework, implementation details, and properties of the Elo rating system implemented in **DSAblitz**.

---

## 1. Core Mathematics

The Elo rating system is a method for calculating the relative skill levels of players in zero-sum games (like 1v1 DSA battles).

### Expected Score Calculation
Given Player A with rating $R_A$ and Player B with rating $R_B$, the probability (expected score) of Player A winning is calculated as:

$$E_A = \frac{1}{1 + 10^{(R_B - R_A)/400}}$$

Similarly, the expected score of Player B is:

$$E_B = \frac{1}{1 + 10^{(R_A - R_B)/400}}$$

#### Key Properties of Expected Scores:
* **Complementary**: The sum of expected scores is always exactly $1.0$ ($E_A + E_B = 1.0$).
* **Logistic Curve**: A rating difference of $400$ points means the higher-rated player is expected to win with a probability of $10/11 \approx 90.9\%$. A difference of $800$ points results in $\approx 99.0\%$.

---

## 2. Rating Updates & Delta Calculations

After a match finishes, ratings are updated based on the actual outcome vs. the expected outcome:

$$R_A' = R_A + \Delta_A$$
$$\Delta_A = \text{round}(K \cdot (S_A - E_A))$$

Where:
* $S_A$ is the actual outcome:
  * **Win**: $S_A = 1.0$
  * **Loss**: $S_A = 0.0$
  * **Draw**: $S_A = 0.5$
* $K$ is the development coefficient (often called the K-factor). DSAblitz uses $K = 32$.

### Rounding & Zero-Sum Consistency
Because rating values must be integers in our schema, we round rating changes to the nearest integer using standard mathematical rounding (`math.Round` in Go).

To keep the system strictly **zero-sum** (no ratings are created or destroyed in a match), the rating gain of one player should ideally match the rating loss of the other ($\Delta_A + \Delta_B = 0$).

#### Edge Cases with Rounding:
Due to discrete rounding, there are rare cases where:
$$\text{round}(K \cdot (S_A - E_A)) + \text{round}(K \cdot (S_B - E_B)) \neq 0$$
*Example*: If $K \cdot (S_A - E_A) = 14.5$ and $K \cdot (S_B - E_B) = -14.5$, Go's `math.Round` rounds both away from zero, yielding $15$ and $-15$ (which sums to $0$). However, if asymmetrical float values occur due to precision limits, a drift of $1$ rating point could occur.
*Mitigation*: Our unit tests verify that for all standard cases, the Elo calculations remain strictly balanced ($\Delta_A + \Delta_B = 0$).

---

## 3. Floor Limits

To prevent players from falling into negative rating spaces, we enforce a hard floor of $0$:

$$R_{\text{final}} = \max(0, R + \Delta)$$

### Non-Zero-Sum Inflation at the Floor
When a low-rated player (e.g. rating $10$) loses to a higher-rated player, their mathematically calculated delta is $-16$.
* Enforcing the floor caps their final rating at $0$ (a net change of $-10$).
* However, the winning player still gains $+16$.
* This introduces **rating inflation** into the ecosystem (a net increase of $+6$ points).
* This is a known limitation of Elo systems with floor boundaries. For our MVP, this inflation is acceptable. In V2, matchmaking pools and season resets will periodically drain excess points.
