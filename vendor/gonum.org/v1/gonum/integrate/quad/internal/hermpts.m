% Copyright (c) 2015, The Chancellor, Masters and Scholars of the University 
% of Oxford, and the Chebfun Developers.
% Copyright (c) 2016 The Gonum Authors
% All rights reserved.
%
% Redistribution and use in source and binary forms, with or without
% modification, are permitted provided that the following conditions are met:
%     * Redistributions of source code must retain the above copyright
%       notice, this list of conditions and the following disclaimer.
%     * Redistributions in binary form must reproduce the above copyright
%       notice, this list of conditions and the following disclaimer in the
%       documentation and/or other materials provided with the distribution.
%     * Neither the name of the University of Oxford nor the names of its 
%       contributors may be used to endorse or promote products derived from 
%       this software without specific prior written permission.
%
% THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
% ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
% WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
% DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR 
% ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
% (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
% LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
% ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
% (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
% SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.


function [x, w, v] = hermpts(n, varargin)
%HERMPTS   Hermite points and Gauss-Hermite Quadrature Weights.
%   HERMPTS(N) returns N Hermite points X in (-inf, inf). By default these are
%   roots of the 'physicist'-type Hermite polynomials, which are orthogonal with
%   respect to the weight exp(-x.^2).
%
%   HERMPTS(N, 'PROB') normalises instead by the probablist's definition (with
%   weight exp(-x.^2/2)), which gives rise to monomials.
%
%   [X, W] = HERMPTS(N) returns also a row vector W of weights for Gauss-Hermite
%   quadrature. [X,W,V] = HERMPTS(N) returns in addition a column vector V of
%   the barycentric weights corresponding to X.
%
%   [X, W] = HERMPTS(N, METHOD) where METHOD is one of 'GW', 'REC', 'GLR', or 
%   'ASY' allows the user to select which method is used. 'GW' will use the 
%   traditional Golub-Welsch eigenvalue method [1], best when n<=20. 'REC' 
%   uses Newton's method with polynomial evaluation via the 3-term 
%   recurrence for Hermite polynomials. 'GLR' uses Glaser-Liu-Rokhlin 
%   fast algorithm which is much faster for large N [2]. 'ASY' uses Newton's 
%   method with polynomial evaluation via asymptotic formula. 'ASY' is the 
%   fastest for N>=200, 'GLR' is the most accurate for nodes close to 0. 
%   By default HERMPTS uses 'GW' when N <= 20, 'REC' for 21<=N<200, and
%   'ASY' when N>=200. 
%
% References:
%   [1] G. H. Golub and J. A. Welsch, "Calculation of Gauss quadrature
%       rules", Math. Comp. 23:221-230, 1969,
%   [2] A. Glaser, X. Liu and V. Rokhlin, "A fast algorithm for the
%       calculation of the roots of special functions", SIAM Journal
%       on Scientific Computing", 29(4):1420-1438:, 2007.
%   [3] A. Townsend, T. Trogdon and S. Olver, Fast computation of Gauss
%       nodes and weights on the whole real line, submitted, 2014. 
%
% See also CHEBPTS, LEGPTS, LAGPTS, and JACPTS.

% Copyright 2015 by The University of Oxford and The Chebfun Developers. 
% See http://www.chebfun.org/ for Chebfun information.
%
% 'GW' by Nick Trefethen, March 2009 - algorithm adapted from [1].
% 'GLR' by Nick Hale, March 2010 - algorithm adapted from [2].

% Defaults:
method = 'default';
type = 'phys';

if ( n < 0 )
    error('CHEBFUN:hermpts:n', 'First input should be a positive integer.');
end

% Return empty vector if n = 0.
if ( n == 0 )
    [x, w, v] = deal([]); 
    return
end

% Check the inputs
while ( ~isempty(varargin) )
    s = varargin{1}; 
    varargin(1) = [];
    if ( strcmpi(s, 'GW') )
        method = 'GW';
    elseif ( strcmpi(s,'glr') )
        method = 'GLR';  
    elseif ( strcmpi(s,'rec') )
        method = 'REC';
    elseif ( strcmpi(s,'asy') )
        method = 'ASY';
    elseif ( strncmpi(s, 'phys', 3) )
        type = 'phys';
    elseif ( strncmpi(s, 'prob', 3) )
        type = 'prob'; 
    else
        error('CHEBFUN:hermpts:input', 'Unrecognised input string; %s.', s);
    end
end

% Three cases: 
% 
%  N <= 20:  Use GW
%  21<=N<200: Use REC
%  N>=200: Use ASY
if ( n == 1 )
    % n = 1 case is trivial
    x = 0; 
    w = sqrt(pi); 
    v = 1;           
    
elseif ( (n < 21 && strcmpi(method,'default')) || strcmpi(method,'GW') )
    % GW, see [1]
    
    beta = sqrt(.5*(1:n-1));              % 3-term recurrence coeffs
    T = diag(beta, 1) + diag(beta, -1);   % Jacobi matrix
    [V, D] = eig(T);                      % Eigenvalue decomposition
    [x, indx] = sort(diag(D));            % Hermite points
    w = sqrt(pi)*V(1, indx).^2;           % weights
    v = abs(V(1, indx)).';                % Barycentric weights
    v = v./max(v);                        % Normalize
    v(2:2:n) = -v(2:2:n);
    
    % Enforce symmetry:
    ii = 1:floor(n/2);  
    x = x(ii);  
    w = w(ii);
    vmid = v(floor(n/2)+1); 
    v = v(ii);
    if ( mod(n, 2) )
        x = [x ; 0 ; -x(end:-1:1)];   
        w = [w, sqrt(pi) - sum(2*w), w(end:-1:1)];
        v = [v ; vmid ; v(end:-1:1)];
    else
        x = [x ; -x(end:-1:1)];
        w = [w, w(end:-1:1)];
        v = [v ; -v(end:-1:1)];
    end
    
elseif ( strcmpi(method,'GLR') )
    % Fast, see [2]
    
    [x, ders] = alg0_Herm(n);             % Nodes and H_n'(x)
    w = (2*exp(-x.^2)./ders.^2)';         % Quadrature weights
    v = exp(-x.^2/2)./ders;               % Barycentric weights
    v = v./max(abs(v));                   % Normalize
    if ( ~mod(n, 2) )
        ii = (n/2+1):n; 
        v(ii) = -v(ii); 
    end
    
elseif ( (n < 200 && strcmpi(method,'default')) || strcmpi(method,'REC') )
    
    [x, w, v] = hermpts_rec( n ); 
    
else
    
    [x, w, v] = hermpts_asy( n ); 
    
end

% Normalise so that sum(w) = sqrt(pi)
w = (sqrt(pi)/sum(w))*w;                  

if ( strcmpi(type, 'prob') )
    x = x*sqrt(2);
    w = w*sqrt(2);
end

end

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%% %%%%%%%%%%%%%%%%%%%%%%% Routines for GLR algorithm %%%%%%%%%%%%%%%%%%%%%%%%%
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

% Driver for 'GLR'.
function [roots, ders] = alg0_Herm(n) 
% Compute coefficients of H_m(0), H_m'(0), m = 0,..,N.

Hm2 = 0; 
Hm1 = pi^(-1/4); 
Hpm2 = 0; 
Hpm1 = 0;
for k = 0:n-1
    H = -sqrt(k/(k+1))*Hm2;
    Hp = sqrt(2/(k+1))*Hm1-sqrt(k/(k+1))*Hpm2;
    Hm2 = Hm1; 
    Hm1 = H; 
    Hpm2 = Hpm1; 
    Hpm1 = Hp;
end

% allocate storage
roots = zeros(n, 1); 
ders = zeros(n, 1);                      
if ( mod(n,2) )
    % zero is a root:
    roots((n-1)/2) = 0; 
    ders((n+1)/2) = Hp;         
else
    % find first root:
    [roots(n/2+1), ders(n/2+1)] = alg2_Herm(H,n); 
end        

% compute roots and derivatives:
[roots, ders] = alg1_Herm(roots, ders); 

end

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

% Main algorithm for 'GLR'
function [roots, ders] = alg1_Herm(roots, ders) 

n = length(roots);
s = mod(n, 2);
N = (n - s) / 2;

% number of terms in Taylor expansion
m = 30; 

% initialise
hh1 = ones(m + 1, 1); 
u = zeros(1, m + 1); 
up = zeros(1, m + 1);

for j = (N + 1):(n - 1)
    
    % previous root
    x = roots(j); 
    
    % initial approx
    h = rk2_Herm(pi/2,-pi/2,x,n) - x;

    % scaling
    M = 1/h;
    
    % recurrence relation for Hermite polynomials
    c1 = -(2*n+1-x^2)/M^2; 
    c2 = 2*x./M^3; 
    c3 = 1./M^4;
    u(1) = 0; 
    u(2) = ders(j)/M; 
    u(3) = .5*c1*u(1);
    u(4) = (c1*u(2) + c2*u(1))/6;
    up(1) = u(2); 
    up(2) = 2*u(3)*M; 
    up(3) = 3*u(4)*M; 
    up(m+1) = 0;
    
    for k = 2:m-2
        u(k+3) = (c1*u(k+1) + c2*u(k) + c3*u(k-1))/((k+1)*(k+2));
        up(k+2) = (k+2)*u(k+3)*M;
    end
  
    % flip for more accuracy in inner product calculation
    u = u(m+1:-1:1);       
    up = up(m+1:-1:1);
    
    % Newton iteration
    hh = hh1; 
    hh(end) = M;    
    step = inf;  
    l = 0; 
    z = zeros(m, 1);
    while ( (abs(step) > eps) && (l < 10) )
        l = l + 1;
        step = (u*hh)/(up*hh);
        h = h - step;
        % powers of h (This is the fastest way!)
        hh = [M ; cumprod(M*h + z)]; 
        % flip for more accuracy in inner product calculation
        hh = hh(end:-1:1); 
    end
    
    % update
    roots(j+1) = x + h;
    ders(j+1) = up*hh;
end

% nodes are symmetric
roots(1:N+s) = -roots(n:-1:N+1);
ders(1:N+s) = ders(n:-1:N+1);

end

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

% find the first root (note H_n'(0) = 0)
function [x1, d1] = alg2_Herm(Hn0, n) 

% advance ODE via Runge-Kutta for initial approx
x1 = rk2_Herm(0, -pi/2, 0, n);

% number of terms in Taylor expansion
m = 30; 

% scaling
M = 1/x1;
% c = log10(n);
% M = 1./x1.^(1-1.25/(c));

% initialise
u = zeros(1,m+1); 
up = zeros(1,m+1);

% recurrence relation for Legendre polynomials
u(1) = Hn0; 
u(3) = -.5*(2*n+1)*u(1)/M^2;
up(1) = 0; 
up(2) = 2*u(3)*M;
for k = 2:2:m-2
    u(k+3) = (-(2*n+1)*u(k+1)/M^2 + u(k-1)/M^4)/((k+1)*(k+2));
    up(k+2) = (k+2)*u(k+3)*M;
end

% flip for more accuracy in inner product calculation
u = u(m+1:-1:1);
up = up(m+1:-1:1);

z = zeros(m, 1);
x1k = [M ; cumprod(M*x1 + z)];
step = inf; 
l = 0;
% Newton iteration
while ( (abs(step) > eps) && (l < 10) )
    l = l + 1;
    step = (u*x1k)/(up*x1k);
    x1 = x1 - step;
    % powers of h (This is the fastest way!)
    x1k = [1 ; cumprod(M*x1 + z)]; 
    x1k = x1k(end:-1:1);
end

% Update derivative
d1 = up*x1k;

end

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

% Runge-Kutta for Hermite Equation
function x = rk2_Herm(t, tn, x, n) 
m = 10; 
h = (tn-t)/m;
for j = 1:m
    k1 = -h/(sqrt(2*n+1-x^2) - .5*x*sin(2*t)/(2*n+1-x^2));
    t = t + h;
    k2 = -h/(sqrt(2*n+1-(x+k1)^2) - .5*x*sin(2*t)/(2*n+1-(x+k1)^2));
    x = x + .5*(k1 + k2);
end
end

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%% %%%%%%%%%%%%%%%%%%%%%%% Routines for ASY algorithm %%%%%%%%%%%%%%%%%%%%%%%%%
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

function [x, w, v] = hermpts_asy(n)
% HERMPTS_ASY, fast algorithm for computing Gauss-Hermite nodes and weights 
% using Newton's method with polynomial evaluation via asymptotic expansions.  
% 
%  x = Gauss-Hermite nodes, w = quad weights, v = bary weights.
% 
%  See [3]. 

[x, w, v] = hermpts_asy0( n );   

if mod(n,2) == 1                              % fold out
    x = [-x(end:-1:1);x(2:end)];
    w = [w(end:-1:1) w(2:end)]; w = (sqrt(pi)/sum(w))*w;
    v = [v(end:-1:1);v(2:end)]; v = v./max(abs(v));
else
    x = [-x(end:-1:1);x];
    w = [w(end:-1:1) w]; w = (sqrt(pi)/sum(w))*w;
    v = [v(end:-1:1);-v]; v = v./max(abs(v));
end

% debug
%tic, exact = hermpts(n); toc
%semilogy(abs(exact-x))
end

function [x, w, v] = hermpts_asy0(n)
% Compute Hermite nodes and weights using asymptotic formula

x0 = HermiteInitialGuesses(n);   % get initial guesses
t0 = x0./sqrt(2*n+1);
theta0 = acos(t0);               % convert to theta-variable
for k = 1:20
    [val, dval] = hermpoly_asy_airy(n, theta0);
    dt = -val./(sqrt(2)*sqrt(2*n+1)*dval.*sin(theta0));
    theta0 = theta0 - dt;                        % Newton update
    if norm(dt,inf) < sqrt(eps)/10, break; end
end
t0 = cos(theta0);
x = sqrt(2*n+1)*t0;                          % back to x-variable

ders = x.*val + sqrt(2)*dval;
%ders = dval;
w = (exp(-x.^2)./ders.^2)';            % quadrature weights

v = exp(-x.^2/2)./ders;               % Barycentric weights
end

function [val, dval] = hermpoly_asy_airy(n, theta)
% HERMPOLY_ASY evaluation hermite poly using Airy asymptotic formula in
% theta-space.

musq = 2*n+1;
cosT = cos(theta); sinT = sin(theta);
sin2T = 2*cosT.*sinT;
eta = .5*theta - .25*sin2T;
chi = -(3*eta/2).^(2/3);
phi = (-chi./sinT.^2).^(1/4);
const = 2*sqrt(pi)*musq^(1/6)*phi; 
Airy0 = real(airy(musq.^(2/3)*chi));
Airy1 = real(airy(1,musq.^(2/3)*chi));

% Terms in (12.10.43):
a0 = 1; b0 = 1;
a1 = 15/144; b1 = -7/5*a1;
a2 = 5*7*9*11/2/144^2; b2 = -13/11*a2;
a3 = 7*9*11*13*15*17/6/144^3;
b3 = -19/17*a3;

% u polynomials in (12.10.9)
u0 = 1; u1 = (cosT.^3-6*cosT)/24;
u2 = (-9*cosT.^4 + 249*cosT.^2 + 145)/1152;
u3 = (-4042*cosT.^9+18189*cosT.^7-28287*cosT.^5-151995*cosT.^3-259290*cosT)/414720;

%first term
A0 = 1;
val = A0*Airy0;

%second term
B0 = -(a0*phi.^6.*u1+a1*u0)./chi.^2;
val = val + B0.*Airy1./musq.^(4/3);

%third term
A1 = (b0*phi.^12.*u2 + b1*phi.^6.*u1 + b2*u0)./chi.^3;
val = val + A1.*Airy0/musq.^2;

%fourth term
B1 = -(phi.^18.*u3 + a1*phi.^12.*u2 + a2*phi.^6.*u1 + a3*u0)./chi.^5;
val = val + B1.*Airy1./musq.^(4/3+2);

val = const.*val;

%% Derivative

eta = .5*theta - .25*sin2T;
chi = -(3*eta/2).^(2/3);
phi = (-chi./sinT.^2).^(1/4);
const = sqrt(2*pi)*musq^(1/3)./phi;

% v polynomials in (12.10.10)
v0 = 1; v1 = (cosT.^3+6*cosT)/24;
v2 = (15*cosT.^4-327*cosT.^2-143)/1152;
v3 = (259290*cosT + 238425*cosT.^3 - 36387*cosT.^5 + 18189*cosT.^7 -...
    4042*cosT.^9)/414720;

%first term
C0 = -(b0*phi.^6.*v1 + b1.*v0)./chi;
dval = C0.*Airy0/musq.^(2/3);

% %second term
D0 =  a0*v0;
dval = dval + D0*Airy1;

% %third term
C1 = -(phi.^18.*v3 + b1*phi.^12.*v2 + b2*phi.^6.*v1 + b3*v0)./chi.^4;
dval = dval + C1.*Airy0/musq.^(2/3+2);

%fourth term
D1 = (a0*phi.^12.*v2 + a1*phi.^6.*v1 + a2*v0)./chi.^3;
dval = dval + D1.*Airy1/musq.^2;

dval = const.*dval;

end

function x_init = HermiteInitialGuesses(n)
%HERMITEINTITIALGUESSES(N), Initial guesses for Hermite zeros.
%
% [1] L. Gatteschi, Asymptotics and bounds for the zeros of Laguerre
% polynomials: a survey, J. Comput. Appl. Math., 144 (2002), pp. 7-27.
%
% [2] F. G. Tricomi, Sugli zeri delle funzioni di cui si conosce una
% rappresentazione asintotica, Ann. Mat. Pura Appl. 26 (1947), pp. 283-300.

% Gatteschi formula involving airy roots [1].
% These initial guess are good near x = sqrt(n+1/2);
if mod(n,2) == 1
    m = (n-1)/2; bess = (1:m)'*pi; a = .5;
else
    m = n/2; bess = ((0:m-1)'+.5)*pi; a = -.5;
end
nu = 4*m + 2*a + 2;
T = @(t) t.^(2/3).*(1+5/48*t.^(-2)-5/36*t.^(-4)+(77125/82944)*t.^(-6) -...
    108056875/6967296*t.^(-8)+162375596875/334430208*t.^(-10));
airyrts = -T(3/8*pi*(4*(1:m)'-1));

airyrts_exact = [  -2.338107410459762           % Exact Airy roots.
    -4.087949444130970
    -5.520559828095555
    -6.786708090071765
    -7.944133587120863
    -9.022650853340979
    -10.040174341558084
    -11.008524303733260
    -11.936015563236262
    -12.828776752865757];
airyrts(1:10) = airyrts_exact;  % correct first 10.

x_init = sqrt(nu + 2^(2/3)*airyrts*nu^(1/3) +...
    1/5*2^(4/3)*airyrts.^2*nu^(-1/3) +...
    (11/35-a^2-12/175*airyrts.^3)/nu +...
    (16/1575*airyrts+92/7875*airyrts.^4)*2^(2/3)*nu^(-5/3) -...
    (15152/3031875*airyrts.^5+1088/121275*airyrts.^2)*2^(1/3)*nu^(-7/3));
x_init_airy = real(x_init(end:-1:1));

% Tricomi initial guesses. Equation (2.1) in [1]. Originally in [2].
% These initial guesses are good near x = 0 . Note: zeros of besselj(+/-.5,x)
% are integer and half-integer multiples of pi.
% x_init_bess =  bess/sqrt(nu).*sqrt((1+ (bess.^2+2*(a^2-1))/3/nu^2) );
Tnk0 = pi/2*ones(m,1);
nu = (4*m+2*a+2);
rhs = (4*m-4*(1:m)'+3)./nu*pi;

for k = 1:7
    val = Tnk0 - sin(Tnk0) - rhs;
    dval = 1 - cos(Tnk0);
    dTnk0 = val./dval;
    Tnk0 = Tnk0 - dTnk0;
end

tnk = cos(Tnk0/2).^2;
x_init_sin = sqrt(nu*tnk - (5./(4*(1-tnk).^2) - 1./(1-tnk)-1+3*a^2)/3/nu);

% Patch together
p = 0.4985+eps;
x_init = [x_init_sin(1:floor(p*n));x_init_airy(ceil(p*n):end)];


if mod(n,2) == 1
    x_init = [0;x_init];
    x_init = x_init(1:m+1); 
else
    x_init = x_init(1:m); 
end

% debug:
%y = hermpts(n);
%semilogy(abs(y - x_init));
%yhalf = -y(m:-1:1);
%semilogy(abs(yhalf - x_init));
end

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%% %%%%%%%%%%%%%%%%%%%%%%% Routines for REC algorithm %%%%%%%%%%%%%%%%%%%%%%%%%
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

function [x, w, v] = hermpts_rec(n)
% Compute Hermite nodes and weights using recurrence relation.

x0 = HermiteInitialGuesses(n);
x0 = x0.*sqrt(2);

for kk = 1:10
    [val, dval] = hermpoly_rec(n, x0);
    dx = val./dval;
    dx(isnan(dx)) = 0;
    x0 = x0 - dx;
    if norm(dx, inf)<sqrt(eps), break; end
end
x = x0/sqrt(2);
w = (exp(-x.^2)./dval.^2)';            % quadrature weights
v = exp(-x.^2/2)./dval;               % Barycentric weights

if mod(n,2) == 1                              % fold out
    x = [-x(end:-1:1);x(2:end)];
    w = [w(end:-1:1) w(2:end)]; w = (sqrt(pi)/sum(w))*w;
    v = [v(end:-1:1);v(2:end)]; v = v./max(abs(v));
else
    x = [-x(end:-1:1);x];
    w = [w(end:-1:1) w]; w = (sqrt(pi)/sum(w))*w;
    v = [v(end:-1:1);-v]; v = v./max(abs(v));
end


end

function [val, dval] = hermpoly_rec(n, x0)
% HERMPOLY_rec evaluation of scaled Hermite poly using recurrence

% evaluate:
Hold = exp(-x0.^2/4); H = x0.*exp(-x0.^2/4);
for k = 1:n-1
    Hnew = (x0.*H./sqrt(k+1) - Hold./sqrt(1+1/k));
    Hold = H; H = Hnew;
end
% evaluate derivative:
val = Hnew;
dval = (-x0.*Hnew + n^(1/2)*Hold);
end