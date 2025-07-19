package router

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	admissionapi "k8s.io/pod-security-admission/api"
	utilpointer "k8s.io/utils/pointer"

	routev1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"

	"github.com/openshift/origin/test/extended/router/h2spec"
	"github.com/openshift/origin/test/extended/router/shard"
	exutil "github.com/openshift/origin/test/extended/util"
)

const h2specDialTimeoutInSeconds = 30

type h2specFailingTest struct {
	TestCase   *h2spec.JUnitTestCase
	TestNumber int
}

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router][apigroup:route.openshift.io]", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithPodSecurityLevel("router-h2spec", admissionapi.LevelBaseline)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(oc.KubeFramework().Namespace.Name)
			if routes, _ := client.List(context.Background(), metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("h2spec", oc)
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should pass the h2spec conformance tests [apigroup:authorization.openshift.io][apigroup:user.openshift.io][apigroup:security.openshift.io][apigroup:operator.openshift.io]", func() {
			isProxyJob, err := exutil.IsClusterProxyEnabled(oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get proxy configuration")
			if isProxyJob {
				g.Skip("Skip on proxy jobs")
			}

			infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster-wide infrastructure")
			if !platformHasHTTP2LoadBalancerService(infra.Status.PlatformStatus.Type) {
				g.Skip("Skip on platforms where the default router is not exposed by a load balancer service.")
			}

			g.By("Getting the default domain")
			defaultDomain, err := getDefaultIngressClusterDomainName(oc, time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")

			g.By("Locating the router image reference")
			routerImage, err := exutil.FindRouterImage(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Locating the canary image reference")
			canaryImage, err := getCanaryImage(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.AsAdmin().Run("adm", "policy", "add-scc-to-user").Args("restricted", "-n", oc.Namespace(), "-z", "default").Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to provide the default namespace SA with access to the restricted SCC")

			err = exutil.WaitForUserBeAuthorized(oc, "system:serviceaccount:"+oc.Namespace()+":default",
				&authorizationv1.ResourceAttributes{
					Namespace: oc.Namespace(),
					Verb:      "use",
					Group:     securityv1.GroupName,
					Resource:  "securitycontextconstraints",
					Name:      "restricted"},
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating h2spec test service config")
			ns := oc.KubeFramework().Namespace.Name
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "h2spec-haproxy-config",
				},
				Data: map[string]string{
					"haproxy.config": `
global
	daemon
	log stdout local0
	nbthread 4
	tune.ssl.default-dh-param 2048
	tune.ssl.capture-buffer-size 1
defaults
	mode http
	timeout connect 5s
	timeout client 30s
	timeout client-fin 1s
	timeout server 30s
	timeout server-fin 1s
	timeout http-request 10s
	timeout http-keep-alive 300s
	option logasap
	option http-buffer-request
	log-format "frontend:%f/%H/%fi:%fp GMT:%T\ body:%[capture.req.hdr(0)]\  request:%r"
frontend fe_proxy_tls
	option http-buffer-request
	declare capture request len 40000
	http-request capture req.body id 0
	log global
	bind *:8443 ssl crt /tmp/bundle.pem alpn h2
	default_backend haproxy-availability-ok
backend haproxy-availability-ok
	errorfile 503 /etc/haproxy/errorfile
	http-request deny deny_status 200
`,
					"errorfile": `
HTTP/1.1 200 OK
Content-Length: 8192
Cache-Control: max-age=28800
Content-Type: text/plain

2wWvUP5ISuTTzmzf27uZ/hGEVQMowYJYgDBZPGj3VY9XEHtdiCILqnw6oMvB95lUtNDPfVh+sEpM
4NbGyxC/hALxe98LaexsWfMgdtrOs0Cre2MwGeL2Vgr68Ju9mTzL3YpYetU09WSesko6RfnqjPyA
b0dsc7XecYeh8XfetC5WgUfsGGhJTKEd80ClFAWv0usTU+qccoG7zkxxTGzw5qzp7L+B4t8Bwgjf
dvFOZZ3cwPowiGg+4iF7rwbBCtOfXgFe/eBVGpP5KtW6hcdf7Wqw/w6Tkf8ZXlKSzT6xLXrq0C73
OrUwvRn+NJl6wbpSOFEvB3Cp19Q0oMTa9+alvPwWZxwXEIi85hT5YVDZsb0pP1hcTOQAsT5LOWzm
mtNcIstM50XZj1hHEhJeixp5gAsrwY1m+Uwm2X6a70NBEtqnP0B04oOIPfTtebORGu1DiJGgntWM
wdk1ReLyDLTS2tISn6ItAwknF0Qk3D5kMqNN2sB1GBcWf7zqTlgB3W2p6I31P2Vt/I+z859JwbIw
3w3AI5UAGSPmguLzzdPrqKa1igzrBcoDvEJnk2O0+39qlJ+Sa2Ko02KjGkl7ZNZJwUAIKMsC5vAl
hV2KFRtRnWa7YzDMuNzoOZezPnIz8zvLVQFVGCSnpu7crAKrrhJD9F/nDBEnLtA5lzJRf32LUYNI
tCs2CHt8guaddJ1U1+lEGLKX3QM0N62MhDQy2lZwAvag8WlW1le+kj0vO1NYCwauzEWZtdHEedGv
E98m9Y4OWDLl4k8uTV0f8vsgwHTCgFcJ8EmWYizi/ykL1kfdR324JiW+3YpH3F8GEp9L7ESkqIns
eXajNzKhagc1e+YM8Xe6SjWDXbdVV9ZSEsgdhK2gy0MQchK2vU1hzUKq4cxDTMJ8k3CAkuG3IFpd
Nyv9eW4aJUSsNv2OzH0iRUaXs3qAefORFQgn8/Qe2c6wSDAI5wHEi7zi/Lick3UVv+7V13zfvcWl
32A2p1Erotjl/tgj4lX60Ci3uRgRBQ/9wR/N9JuH0A4ynn0uBaS1M/Qpbmz78/oeXQgCEnUCEA4k
DXYvXl6o+dEfJkuUYMIAH4wadtmdf+DSH9oOPvBFSM93X8BF21SSDeb8K+YfIi6+Ivzll+5jcNoi
uUryTyp1don75Zk6CT7b2m1o514MS68ulcNI4g36GpaS44rnuvQGyacdau6NabzgR0Q/3n9kOlFE
IOse9+eUEmR6KXZ/DuoeT7M2+Qul4uNwJz8i2RrF7mAToB3k0qdA8fO2munXXWoGr77vSkEDdJeq
ihFBQ60KNZeZh4x18uAxYigNrYfWjmIFAdzQd9XpsGL7iHYmjyHUQQabzFirJdeS4w4hZoSznA5m
1CtCvRtAT8RPoiUPSqKU3QtH46iNGusjRoRfCj7ynrmeqeDqkw4H34CrnkolqT1hDqvaqZIyJo50
D3MGeURwMM6DYjWKOaVJaQDbXC8Ahb67+1nKUEyEaLKkfTh8GPGOnmBiWub/Y/N3AL9TEuihw9KP
NtjZQ82jL32NqdSdwKDXmE2SMmElUOY6fVFEGDVdgx9eJbeMaiSwXLTtUFxAxsO1wY5jDf8Cr97w
P8tLv1CPcec381Y2jAD0CgkGaa1u0VTj0jLFIwZK2faeKa3VJrB7ldYD74+PwiIgfl9nbvxlC8KN
5RTd7ThSGRQ+N7zpjRdaoftafUcFj6G/O/QrbhPxLZHcHG+zBGt/Fkr1lswfjiDsHHSM1ZyLiuny
ZqFBSSjL8X+NOa76tUq414UrZZ85w6nDTkzitXb36x8TEgfaoipUZJVNQ8smjE3bO9wB1zyzYXh7
vDQe9p3GfRN223tJKGhXZ1SewOqoZsEWTogk6FFxngAyYb6jfqFFChe9gSrjS54+WUm0HyvSGuks
q/NwwvgI69cXqPZL6eXpgAAwFbt366HbGDHcKaG02fmuBNdhguw1BuF3EaBiPF2beQvYx9GPyzua
VDTflywUGXI3JixRbwT0TgXDIX+2FceA5NcyGQLjwF5CpDH650PaholA3dUif8Blls+FpJ74UdK1
Ws+mG/UaBZ31hLHKqHI986G3PSxEWYyrF6vL6+CuNfet/SYh7AMRWK93Rkb3/N8GPosuFPaBNZLR
EBSHW9HUTP0viNWDupGx8mmncAUb9HLjqcFJoWGqZjVKaYe8J3NwvaL1P8+/v7ckpLUzOgiZVake
azDZDBoEfqFp/EGwnwm/KsnCQZ/I0aqrVW8T3AjUyFRIBw+rYLLGC2oIiUDH5ccvYhDY1epYS3C/
qW+mWa1XNz0Aat+7LFoMt4BG3319S/fqApIRMq3rcoegfPhGSI9CBoNnLCxz/GHnlSxstCIQdnMJ
xwWBgvHuVb84bHfsRknUQX7g5s7xf9UK06TXRmYG+lb70Trkb0EZKzT17IMIOnZk4BCJkX08YK88
C1rP68EjSdLSRiln3EPJ6kuNVFct077SfDG3SiLldx/VsZGSFzqWv69Qdb82wI+v5FcV3TZkrZAP
mhHJEWFaWvtEMyc7TtNI+0XhME96RIscBSLtoaRRV8CbMSJ8uanfox5LFId0gD4kfWiGtirj9/1/
GnAUoMhFeipQ8mYKu2zwOFsDVmWzC10uNyorY4qg/WBJ6A3asEcHIUVkmOnakPkRipKTKxFYlXjF
1Jau+KsvHTvWxOP/LTDipJjxwQWBzDEmUHOQJJrHQG/grmOPFB891bcFRLWzYSuSYCSLetA8HlCK
m9Bxit43AUhLeeUoVHroflvyHhI1LT2k6crEz4g/bdLMi7ncbtCmB88k6UYXUaXKL2YlzxRp+cWA
nxeR63cR2RXeqUVdO3GqgAFKHFw96lgbF74qBc9AE5r5juzvT6qoHq7sHNJ31VhA6cASdIio+H+D
O2sb8xvGyuCfydIHgJoRc2ilhVsMPwEoMsCrp1MRWE5tLgkn0uH5RjV1K1yDYY0PivgJYbBtjOhx
mcaaa+P8jHc7J/Q6rI6BCjehbOwFY7dbCjcBJ8y39yNvDFwtj53UxMiWoRSwNO8ICJNFwm1dXjUa
gJ/+g6q0U4qf0nL5f/whHCsY8qdD9Jj9qcRjvSNaiP/l44ETGA2bc+/33cdZZImYAw54nfoN1UPx
hcvP3dsol6SaHgGOvZV0R6sapasMbIuFOkAXEVjn75E1dnWoom2k/cWH1gCxStYKUE4ilsMi+Smb
ejw1wXXJ4IG/861DPEAfrhwXO5nBppSClyf8ASMI+EjJmEO9o9b+hvKST0lN/+qnXfgzyirrhjSH
B8mMyArxcZo3+avdi1hC8VgNsRpR9aC7Sim9v8gjMfVg0qvIcDPjfvozyXhiEhrc7T+GDqk6Ledv
lOwTMw+i5UlrEEeJXDp8Ae8dQ1i/aLN/J7bR6LI9off7egiSIgnoOaUJl5LfvHqzFJsbjpSrm9U9
hrhs9ChG6Qa1VsB/cvoaLwbzXi3XcbPue8DuNrgTP4CcP7KtiiS+NM+n0nRKEk9y7eeSfjXI5pE7
6JFIdYs2qXFLtc+SuBq4M2dtKySiOr27gi59sbgr/OlWl+JQDNKPZ3XFM9nsoNpD3QU5Ye0DKzrI
rJh5Q/Gt3fQg91sFiB76kkpsQ88GQ/kgui9jadTYZcRmz/vQkoiQShX0xhdbkmwQgocnNO9IkZy+
vua906n5skPPQIpaZOPuIxBoHE/1y+Ap2ofezIBj9p/HNv5Aolc1TL0eY5dPabXWwab/4vutMKos
MKAbI1Gow+RyptiZsau72g/IicWTIpBbveRnbiDWTmw2uwLus4asSanzWjZnlNyy0MIVK0uZRNVn
NBKCXH2VbYMyPIvN9CQbCl7/VnL4qPC8sxkJL28ZtwW881Kn79k49Go7FXZn/go1hdig8av4h+JZ
cHw+bjsNKe3Mr6JvyLIpkvsBFL3TGRQkEy/me6V2HI8dl3RoryJy3SiE8G5uXlKXJywYOaCoIUIp
2uyalKb2YNaZFc6xHjputeIegC4zJh6KmKK8H4n92/qn33DK813xaFpcQWh6HfTL33V1xn6x93jX
x40RmHxbslHN0DYbYcK8fDEdvHfAY/zzKpvXg1TsKYuW8tyeXWL5NjfGND7XliJCo/GIj0dAyWro
IkLvv7XqnAUvLyH+Kd1LBzMa+1Q6luGSQaYaw1Uwioi0+W8VP/vd2MZifv/M+Fg9jXQ0YAPxvnqw
dMNjVq+kCJY9wjwBpgEOdXte5cZebR4b9Zyn0DRFzb4levpCF0bjmJcbzgE/doh8c+qfCIxK57/l
j37u34+y4OjnTeqm991+jnzqjHP9Dr96IjRRVh268Hgqymx670MolqAFlb7Fazwi/+3n4wH6oIjj
cbgFVrsOH0KFnLKf3QFOA2Rr/x+ycY8e0A3Br90AjEzHBsbV2LCpmcB5JaFxQG3K8IGXP2O3h7jP
yXHLPG/Euu0CTN4TlDNl45Ppk2GY48jGb6bdhJjV/qeL49y9wSghFmnGlXkbOxZ/JqI2QeIXleAe
xeVcdnCF9d3mEE0POtHvh4/nF3SS6IwqQd9qtiNLvDrCuhLJCTfowCfTm0WzpNJmaXxrKG4jyUJG
IpVcQSKulIDwkgt66V/PtbgE/2V+4+EvYgP5uM8tf7AAskxlnqB5L81Ph/0zsumrqLUsX1gTONCW
Hqf0cPJlALcHY/FaKq3sZl3J/BoIygIR2IwMeOQCEprt46RsJeY8AAWEk0p9eDoiX7eniV8YFes9
mNUXxHyg1GYzRtbXv0Ua/TomdZwFVhOYGb2SeVCDmzmjPcWLnLZ8949jbHIKIvKgkYgFF5qrtukA
PcPkKGAbzAUpiWr7zn8pp1emm3YRhzvYVJ2gNMtxHZkRg6uNAbt/mF1BqIS8ODtTUUo4+gC/RGYF
bgJryFrYBuFihZLOSXV0T6KNcp/04xRTXI63nfGuJaY0iSoPI3mbeulgxMIFAoALb3nQ9z0bVSzT
Lf6jPmaeM379NQ0bg0IoF+lrRYNTOAE5LssUrDTO8EV402wulLU0MR3bKKkt4jvp04/GpIjn9xmJ
3ZuWjxjvyZGjlaGT/BgsAgi/MuNN1Syty0Pzw8cJUWAogcak/2Xt7cY0+xTWtk7JHy9npv0hNzaw
mpt6NM0Yk4wqMDE9VL8G5P302eAYv11/ZlRM9yDUmTr15wwEc2J0koLqulN96VwMekGsPMi1makl
JpcHjgSuuM4CrD8sd6L8K6IyZWyGBmWV4JQ2Sd4lGvuzxf9+5pS3Q2Iq6QqPzW6rBa9GUAufvtI0
cR+JxqDOwCEd9IwaDq1mvLFUqlfvlGgyj1GrOYMJMMjBa/ErFtnsFL2rzO9g1QkHtErTND50VM8C
IdAybJLV4DOUwzOK3NSElr4Wej8K0Lfbwe4R3KzE4vRc+mO1ZesiPyfM7VsR7dN2NRDTTqWF7dXn
jrCpI2Pwz/BSwbtNvKnVrELydJYqQZ4YN0Kgkb5ZQ+Ei23t+X6IjRNTY576q5BtmNw9MEV70/b4w
Ac0ArzOfp+PbLaC6WdjxzI/AdpZJ5RSBo3w5PY+3P8IG4tz1UyKMhvCtA/xBGTu77C83a0R696aL
kMA5RhYjlCdm73+BMTLp17jXM+j5ek8pt0l5beEWOQSQQuzowiyPwfyp3c77A+3OsuK1dIdTpxh4
EeGLY1UuMQla1ugZODWHac42h6uBftP7Q77qKbCQHHB6G7HlH8xIJp6YfoBbqeQuMhrZrbeWGMpE
XGHizQFlsiHAniPfcY+XaCE4sgW+2gAlR6ESkO3DnGFnyejMspfa+BDdZBfuUO1JNWQwOtlooicQ
JXbSKAVrfDTsFrerk1LJkuhCvIGINt7D+9i9/t+twgA834ObDzb89dpWJAiFV1JtfJW4DGTKga6I
850NJW8/GP4l/hqH0EH9jSDXgjdhS0716/nEjXnwZ0rsHLfGq1AaMUHv972wv+3TA188kzlk7fRr
wuJbuLpwVqp/H1LNueJu+/lzFQoh9eeboguENZNIoZQ7cD0pINwHdeyhXZDomaxHnIrxiZmy72P/
aNkruB+Kf7evbRHzPNZAWkie/PwDrAsPLpeiTuK3nhpd/XIfmnNXZtt1X53MJHRwDMl00ze7lXwn
37Pm2dYsZo2f20cIuVrzyOPv9f9y2y92UAJ6VvPxHjci2lQupmdn/D7kdeF44nZWUMRkvnHW+Lxj
NYHuwwX6sOoKavnmVALOhYk9mukP4pNliuvcJmuhJxaI9oQah8encM2WA8Z7s61Xf1Gk2luMH709
0EX6VvPrNLFUY7xJJsXT191vyrg6Wu5Yd2ZIFXrCgKBLfHumvO3NE+YE+LKK6xrH7Urk9trmKJKt
sfsgmIz8xj4D59tlIsgKZfwGsIbIlachpjhXM9jNdOSe5k2tHNdnh1OvBJvOIqKSp4uVlHZnLUMZ
07rzxr9wdzU4ihaUgvreVpar6vnNYuj/TTDRP0FcBay0IuPunVhX9Wel5ga+NWIV9srCmzsJN7/d
puvaV9sb5dc0M0klEq41bMKDFd86YKifRhwagol5OAHTPjvIqZ9WOr/7XVuxAtOG0l1ohgrKTtfV
jw4KZCd+zIazzwuA0ItCENMmAm2Xppqy1T0Uu7gql3b8XAtsk+IhQw+L8H/oJtt/vaRSnbfTS02N
umm7CcneYyHT1FiuMfm5rkHee7rPR+YiDXlnkrTjd6HaBk3a/mEf0amzsMH9s4FzQRLbYPcXZrfi
ah18pV5ZlcfsC1kmM+wBbxCjxoUcV2DyeGiMdQo2Pif9LpPXOo6SE9a4lDovQF5brB6z9MGUZlKf
n+bQ1SVZxu4ArWLnbmXrgHzz+APsWh6VBfCw0MT8oP7uzB6tzIP1RCm7uKgb1Hi2f8f4DympfW4r
K3/H/5c3foZqlZDSDCGv3amzwkSZ3VsWHPrGFa0jLkTweBf+8UyzRIdoceDI7Ovg9cOiVf4bVqA/
B4DavbV6xOAbHloEJTIEI54epi2CEFnAvpJUgr+uWkgQbSTJVmXUWw0s6gv+2sbeNYYz169c1ScP
U1afX80IXtL0iq7sQjbEPfOg9hWbHQWoAaSgLT0mvGkMHn8eKUBFdvF2paNOfU47OirGz1ifdRZe
9BgBR6glFDlp5g99K6PXxADoy+nHAKnzxlWuxjfMoXgcIWpmIXad+vi3m7J48Z8xAaN8/657UjNW
JmYHVjst8m+Q14lyMJfFj1Q+9FjyKTGwSjryd5dUJacyGrg3mli2v99KnTsOjY1Wm6//G5dcuRIT
IceUARlCKNONQVe3tM4LoIglqTipwfzLwjDfb223BAfNt41otmZ3VGM8yesZQmAnokKhcErDTrw3
g9aoCI6OlrtLrTx3V1k7qW9INLZXspvhU4CalQdWvugpH4prAQO6FeFLCu66/KIL9FZoe5n66UBz
TFR2ih8dKo1JV/aLuqjpsZ+l4lNal4vnqgaLUehC7j1zQAiLD585VMuEliJSmES8wHL3nt5JUKoe
2Y6+aRDQqYUZsEhnPQ1H+0AT5LHOh6P1576m52Bp2tczVjN2K6Hgw+koDUmZj7YUj1stzjKso5rM
0zRAppa9g4XJSDnjaBFdYcRmWZ+PE/sjXzcu1eNtttlJqmYqO4dMGHiffoBIvz9nvqn8eZIRMPdt
D1/ykxN6Cbl42Ox9WTSIZncj6LbhB/5dT12DdCtedx7ljDcGVQm30HbB5GSYWYuWphJSJ0YWX8O+
lW8A3Qy0Vnu2EZUsNKBzgSbws63t2xrizMq0eRkMkHL8L4OUFKenwro6m0PJcuPhTBhVN0ek73vl
YVdXRPoPejw6wPeETZ6ObnCFqySDsycqyIwYXmxFNw3aYiTjFls2i+BZ6lGManDeJ/U/VKdrJt74
Ua3HXuQXe9z/uOBdmiWPBuIA79uzt3C/g5hTFt3L4Q25aRMRXIQkrtRRfP6AEyKJmAUY1hwyIJQV
+HVW+djWL9nO1/REKbJcGPmQwscoH9YYrP4XpLaXbWV/XbuCsyPzW+QKqUinMIX3LlAIYgJp+pyb
m2/3So5gYJkPZxx4UxVrqxAkKhSkQVHvv6Rvj6LkdomEfA76eWKxxvksde+zZkD2ZcWMg0obX1Ox
BFNBRELPe53ZdLKWpf2Sr96vRPRNw
`,
				},
			}

			_, err = oc.AdminKubeClient().CoreV1().ConfigMaps(ns).Create(context.Background(), configMap, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating h2spec test service h2spec-haproxy pod")
			haProxyPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "h2spec-haproxy",
					Labels: map[string]string{
						"app": "h2spec-haproxy",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:   routerImage,
							Name:    "haproxy",
							Command: []string{"/bin/bash", "-c"},
							Args: []string{
								"set -e; cat /etc/serving-cert/tls.key /etc/serving-cert/tls.crt > /tmp/bundle.pem; haproxy -f /etc/haproxy/haproxy.config -db",
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8443,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							ReadinessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(8443),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
							},
							LivenessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(8443),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: utilpointer.Bool(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/etc/serving-cert",
									Name:      "cert",
								},
								{
									MountPath: "/etc/haproxy",
									Name:      "config",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "h2spec-haproxy-config",
									},
								},
							},
						},
						{
							Name: "cert",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "serving-cert-h2spec",
								},
							},
						},
					},
				},
			}

			_, err = oc.AdminKubeClient().CoreV1().Pods(ns).Create(context.Background(), haProxyPod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating h2spec test service object")

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "h2spec-haproxy",
					Annotations: map[string]string{
						"service.beta.openshift.io/serving-cert-secret-name": "serving-cert-h2spec",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "h2spec-haproxy",
					},
					Ports: []corev1.ServicePort{
						{
							Port:       8443,
							Name:       "https",
							TargetPort: intstr.FromInt(8443),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			}

			_, err = oc.AdminKubeClient().CoreV1().Services(ns).Create(context.Background(), service, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating h2spec test service h2spec pod")

			h2specPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "h2spec",
					Labels: map[string]string{
						"app": "h2spec",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "h2spec",
							Image:   canaryImage,
							Command: []string{"sleep"},
							Args:    []string{"infinity"},
						},
					},
				},
			}

			_, err = oc.AdminKubeClient().CoreV1().Pods(ns).Create(context.Background(), h2specPod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			e2e.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(context.TODO(), oc.KubeClient(), "h2spec-haproxy", oc.KubeFramework().Namespace.Name))
			e2e.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(context.TODO(), oc.KubeClient(), "h2spec", oc.KubeFramework().Namespace.Name))

			shardFQDN := oc.Namespace() + "." + defaultDomain

			// The new router shard is using a namespace
			// selector so label this test namespace to
			// match.
			g.By("By labelling the namespace")
			err = oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "type="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating routes to test for h2spec compliance")
			h2specRoute := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name: "h2spec-passthrough",
					Labels: map[string]string{
						"app":  "h2spec-haproxy",
						"type": oc.Namespace(),
					},
				},
				Spec: routev1.RouteSpec{
					Host: "h2spec-passthrough." + shardFQDN,
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(8443),
					},
					TLS: &routev1.TLSConfig{
						Termination:                   routev1.TLSTerminationPassthrough,
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
					},
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   "h2spec-haproxy",
						Weight: utilpointer.Int32(100),
					},
					WildcardPolicy: routev1.WildcardPolicyNone,
				},
			}

			_, err = oc.RouteClient().RouteV1().Routes(ns).Create(context.Background(), h2specRoute, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating a test-specific router shard")
			shardIngressCtrl, err := shard.DeployNewRouterShard(oc, 10*time.Minute, shard.Config{
				Domain: shardFQDN,
				Type:   oc.Namespace(),
			})
			defer func() {
				if shardIngressCtrl != nil {
					if err := oc.AdminOperatorClient().OperatorV1().IngressControllers(shardIngressCtrl.Namespace).Delete(context.Background(), shardIngressCtrl.Name, metav1.DeleteOptions{}); err != nil {
						e2e.Logf("deleting ingress controller failed: %v\n", err)
					}
				}
			}()
			o.Expect(err).NotTo(o.HaveOccurred(), "new router shard did not rollout")

			g.By("Getting LB service")
			shardService, err := getRouterService(oc, 5*time.Minute, "router-"+oc.Namespace())
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(shardService).NotTo(o.BeNil())
			o.Expect(shardService.Status.LoadBalancer.Ingress).ShouldNot(o.BeEmpty())

			host := "h2spec-passthrough." + shardFQDN

			// ROUTER_H2SPEC_SAMPLE when set runs the
			// conformance tests for N iterations to
			// identify flaking tests.
			//
			// This should be enabled in development every
			// time we consume any new version of haproxy
			// be that a major, minor or a micro update to
			// continuously validate the set of test case
			// IDs that fail.
			if iterations := lookupEnv("ROUTER_H2SPEC_SAMPLE", ""); iterations != "" {
				n, err := strconv.Atoi(iterations)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(n).To(o.BeNumerically(">", 0))
				runConformanceTestsAndLogAggregateFailures(oc, host, "h2spec", n)
				return
			}

			testSuites, err := runConformanceTests(oc, host, "h2spec", 10*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(testSuites).ShouldNot(o.BeEmpty())

			failures := failingTests(testSuites)
			failureCount := len(failures)

			g.By("Analyzing results")
			// https://github.com/haproxy/haproxy/issues/471#issuecomment-591420924
			//
			// 5. Streams and Multiplexing
			//   5.4. Error Handling
			//     5.4.1. Connection Error Handling
			//       using source address 10.131.0.37:34742
			//       × 2: Sends an invalid PING frame to receive GOAWAY frame
			//         -> An endpoint that encounters a connection error SHOULD first send a GOAWAY frame
			//            Expected: GOAWAY Frame (Error Code: PROTOCOL_ERROR)
			//              Actual: Connection closed
			//
			// 6. Frame Definitions
			//   6.9. WINDOW_UPDATE
			//     6.9.1. The Flow-Control Window
			//       using source address 10.131.0.37:34816
			//       × 2: Sends multiple WINDOW_UPDATE frames increasing the flow control window to above 2^31-1
			//         -> The endpoint MUST sends a GOAWAY frame with a FLOW_CONTROL_ERROR code.
			//            Expected: GOAWAY Frame (Error Code: FLOW_CONTROL_ERROR)
			//              Actual: Connection closed
			//
			// 147 tests, 145 passed, 0 skipped, 2 failed
			knownFailures := map[string]bool{
				// In HAProxy 2.2 these two tests
				// fail/flake:
				//
				//    "http2/5.4.1.2": true,
				//    "http2/6.9.1.2": true,
				//
				// Using HAProxy 2.6 and 2.8 I have
				// temporarily run this test setting
				// ROUTER_H2SPEC_SAMPLE=100 (which
				// would ordinarily be 1) and I have
				// not observed either of these
				// failures.
			}
			for _, f := range failures {
				if _, exists := knownFailures[f.ID()]; exists {
					failureCount -= 1
					e2e.Logf("TestCase ID: %q is a known failure; ignoring", f.ID())
				} else {
					e2e.Logf("TestCase ID: %q (%q) ****FAILED****", f.ID(), f.TestCase.ClassName)
				}
			}
			o.Expect(failureCount).Should(o.BeZero(), "expected zero failures")
		})
	})
})

func failingTests(testSuites []*h2spec.JUnitTestSuite) []h2specFailingTest {
	var failures []h2specFailingTest

	for _, ts := range testSuites {
		for i := 0; i < ts.Tests; i++ {
			if ts.TestCases[i].Error != nil {
				failures = append(failures, h2specFailingTest{
					TestNumber: i + 1,
					TestCase:   ts.TestCases[i],
				})
			}
		}
	}

	return failures
}

func runConformanceTests(oc *exutil.CLI, host, podName string, timeout time.Duration) ([]*h2spec.JUnitTestSuite, error) {
	var testSuites []*h2spec.JUnitTestSuite

	if err := wait.Poll(2*time.Second, timeout, func() (bool, error) {
		// Error message will read as:
		//   <date>: INFO: lookup <host>: no such host, retrying...
		if _, err := net.LookupHost(host); err != nil {
			e2e.Logf("%v, retrying...", err)
			return false, nil
		}

		g.By("Running the h2spec CLI test")

		// this is the output file in the pod
		outputFile := "/tmp/h2spec-results"

		// h2spec will exit with non-zero if _any_ test in the suite
		// fails, or if there is a dial timeout, so we log the
		// error. But if we can fetch the results and if we can decode the
		// results and we have > 0 test suites from the decoded
		// results then assume the test ran.
		output, err := e2eoutput.RunHostCmd(oc.Namespace(), podName, h2specCommand(h2specDialTimeoutInSeconds, host, outputFile))
		if err != nil {
			e2e.Logf("error running h2spec: %v, but checking on result content", err)
		}

		g.By("Copying results")
		data, err := e2eoutput.RunHostCmd(oc.Namespace(), podName, fmt.Sprintf("cat %q", outputFile))
		if err != nil {
			e2e.Logf("error copying results: %v, retrying...", err)
			return false, nil
		}
		if len(data) == 0 {
			e2e.Logf("results file is zero length, retrying...")
			return false, nil
		}

		g.By("Decoding results")
		testSuites, err = h2spec.DecodeJUnitReport(strings.NewReader(data))
		if err != nil {
			e2e.Logf("error decoding results: %v, retrying...", err)
			return false, nil
		}
		if len(testSuites) == 0 {
			e2e.Logf("expected len(testSuites) > 0, retrying...")
			return false, nil
		}

		// Log what we consider a successful run
		e2e.Logf("h2spec results\n%s", output)
		return true, nil
	}); err != nil {
		return nil, err
	}

	return testSuites, nil
}

func runConformanceTestsAndLogAggregateFailures(oc *exutil.CLI, host, podName string, iterations int) {
	sortKeys := func(m map[string]int) []string {
		var index []string
		for k := range m {
			index = append(index, k)
		}
		sort.Strings(index)
		return index
	}

	printFailures := func(prefix string, m map[string]int) {
		for _, id := range sortKeys(m) {
			e2e.Logf("%sTestCase ID: %q, cumulative failures: %v", prefix, id, m[id])
		}
	}

	failuresByTestCaseID := map[string]int{}

	for i := 1; i <= iterations; i++ {
		testResults, err := runConformanceTests(oc, host, podName, 10*time.Minute)
		if err != nil {
			e2e.Logf("%s", err.Error())
			continue
		}
		failures := failingTests(testResults)
		e2e.Logf("Iteration %v/%v: had %v failures", i, iterations, len(failures))

		// Aggregate any new failures
		for _, f := range failures {
			failuresByTestCaseID[f.ID()]++
		}

		// Dump the current state at every iteration should
		// you wish to interrupt/abort the running test.
		printFailures("\t", failuresByTestCaseID)
	}

	e2e.Logf("Sampling completed: %v test cases failed", len(failuresByTestCaseID))
	printFailures("\t", failuresByTestCaseID)
}

func getHostnameForRoute(oc *exutil.CLI, routeName string) (string, error) {
	var hostname string
	ns := oc.KubeFramework().Namespace.Name
	if err := wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
		route, err := oc.RouteClient().RouteV1().Routes(ns).Get(context.Background(), routeName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Error getting hostname for route %q: %v", routeName, err)
			return false, err
		}
		if len(route.Status.Ingress) == 0 || len(route.Status.Ingress[0].Host) == 0 {
			return false, nil
		}
		hostname = route.Status.Ingress[0].Host
		return true, nil
	}); err != nil {
		return "", err
	}
	return hostname, nil
}

func h2specCommand(timeout int, hostname, results string) string {
	return fmt.Sprintf("ingress-operator h2spec --timeout=%v --tls --insecure --strict --host=%q --junit-report=%q", timeout, hostname, results)
}

func (f h2specFailingTest) ID() string {
	return fmt.Sprintf("%s.%d", f.TestCase.Package, f.TestNumber)
}

func lookupEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}
