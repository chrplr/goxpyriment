import numpy as np
import pandas as pd
import seaborn as sns

import sys

CSV_FILE=sys.argv[1]

df = pd.read_csv(CSV_FILE)

df = df.query("Type == 'Opto1")


sns.histplot(np.diff(df.Onset))

sns.histplot(df.Duration)
