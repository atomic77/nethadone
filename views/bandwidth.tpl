<div class="p-3 row">

    <h3>Data Usage</h3>

    <table class="table">
        <thead>
            <tr>
                <th scope="col">#</th>
                <th scope="col">SrcIp</th>
                <th scope="col">DestIp</th>
                <th scope="col">Bytes</th>
            </tr>
        </thead>
        <tbody>
            {{ range .BandwidthList }}
            <tr>
                <th scope="row"></th>
                <td>{{ .SrcIpAddr }}</td>
                <td>{{ .DestIpAddr }}</td>
                <td>{{ .Bytes }}</td>
            </tr>
            {{end}}

        </tbody>
    </table>

</div>