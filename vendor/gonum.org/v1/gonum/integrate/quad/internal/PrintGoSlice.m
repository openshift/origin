% Copyright ©2016 The Gonum Authors. All rights reserved.
% Use of this source code is governed by a BSD-style
% license that can be found in the LICENSE file.

function PrintGoSlice(a)
      fprintf('[]float64{')
      for i = 1:length(a)
            fprintf('%1.16e, ',a(i))
      end
      fprintf('}\n')
end