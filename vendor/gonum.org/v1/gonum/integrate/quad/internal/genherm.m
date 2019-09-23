% Copyright ©2016 The Gonum Authors. All rights reserved.
% Use of this source code is governed by a BSD-style
% license that can be found in the LICENSE file.

clc
clear all
close all

% Generate herm points
min = 216;
max = 216;
fprintf('xCache = [][]float64{\n')
for i = min:max
    [x,w] = hermpts(i);
    fprintf('{')
    for j = 1:i-1
       fprintf('%1.16e, ',x(j)) 
    end
    fprintf('%1.16e},\n',x(i)) 
end
fprintf('}\n')

fprintf('wCache = [][]float64{\n')
for i = min:max
    [x,w] = hermpts(i);
    fprintf('{')
    for j = 1:i-1
       fprintf('%1.16e, ',w(j)) 
    end
    fprintf('%1.16e},\n',w(i)) 
end
fprintf('}\n')
